$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "missing candidate install package: $installZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'python is required for local OpenAI-compatible E2E provider' }
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw 'git is required for Runtime ChangeSet E2E' }

function Write-Utf8Json([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    $json = $Value | ConvertTo-Json -Depth 40
    [IO.File]::WriteAllText($Path, $json, [Text.UTF8Encoding]::new($false))
}

function Invoke-Runtime([string]$Root, [string[]]$RuntimeArgs) {
    $runtime = Join-Path $Root '.code-harness/bin/codea-dcep-tools.exe'
    if (-not (Test-Path $runtime -PathType Leaf)) { throw "packaged Runtime missing: $runtime" }
    Push-Location $Root
    try {
        $output = (& $runtime @RuntimeArgs 2>&1 | Out-String)
        $exit = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    if ($exit -ne 0) { throw "Runtime failed exit=$exit args=$($RuntimeArgs -join ' '):`n$output" }
    return $output
}

function Initialize-AnalysisScenario([string]$Root, [string]$RunID) {
    $request = Join-Path $Root ".code-harness/runs/$RunID/requests/change-set-request.json"
    Write-Utf8Json $request ([ordered]@{ runId=$RunID; baseRef='HEAD'; includeWorkingTree=$true })
    $null = Invoke-Runtime $Root @('analysis','snapshot','--input',".code-harness/runs/$RunID/requests/change-set-request.json")
}

function Write-AnalysisCertifyRequest([string]$Root, [string]$RunID) {
    $snapshot = Get-Content (Join-Path $Root ".code-harness/runs/$RunID/analysis/change-set.json") -Raw | ConvertFrom-Json
    $request = Join-Path $Root ".code-harness/runs/$RunID/requests/analysis-certify-request.json"
    Write-Utf8Json $request ([ordered]@{
        runId=$RunID
        snapshotPath=".code-harness/runs/$RunID/analysis/change-set.json"
        snapshotSha256=[string]$snapshot.snapshotSha256
        proposalPath=".code-harness/runs/$RunID/requests/change-analysis-proposal.json"
        intent=[ordered]@{ mode='FULL' }
    })
}

function Assert-AnalysisHardStop([string]$Root, [string]$RunID) {
    Write-AnalysisCertifyRequest $Root $RunID
    $runtime = Join-Path $Root '.code-harness/bin/codea-dcep-tools.exe'
    Push-Location $Root
    try {
        $output = (& $runtime analysis certify --input ".code-harness/runs/$RunID/requests/analysis-certify-request.json" 2>&1 | Out-String)
        $exit = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    if ($exit -eq 0) { throw "Reviewer failure incorrectly certified analysis for $RunID" }
    foreach ($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) {
        if ($output -notmatch [regex]::Escape($marker)) { throw "hard-stop output for $RunID missing $marker`n$output" }
    }
    foreach ($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json','review.md')) {
        if (Test-Path (Join-Path $Root ".code-harness/runs/$RunID/$authority")) { throw "hard-stop path $RunID published authority artifact: $authority" }
    }
    return $output
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($installZip)
try {
    $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\\','/') })
} finally {
    $zip.Dispose()
}
foreach ($required in @('.opencode/agents/reviewer.md', '.opencode/commands/harness-review-reviewer.md', '.opencode/tools/codea-reviewer-submit.ts')) {
    if ($entries -notcontains $required) { throw "package missing OpenCode host resource: $required" }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task2-e2e-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force

$reviewerPath = Join-Path $fixture '.opencode/agents/reviewer.md'
$commandPath = Join-Path $fixture '.opencode/commands/harness-review-reviewer.md'
$toolPath = Join-Path $fixture '.opencode/tools/codea-reviewer-submit.ts'
$manifestPath = Join-Path $fixture '.code-harness/RELEASE-MANIFEST.json'
$reviewer = Get-Content $reviewerPath -Raw
$command = Get-Content $commandPath -Raw
$manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
if ($reviewer -notmatch '(?m)^mode:\s*subagent\s*$') { throw 'Reviewer is not registered as OpenCode subagent' }
if ($command -notmatch '(?m)^agent:\s*reviewer\s*$' -or $command -notmatch '(?m)^subagent:\s*true\s*$') { throw 'Reviewer command is not pinned to independent subagent delegation' }
if ($manifest.hostAgents.reviewer.path -ne '.opencode/agents/reviewer.md' -or $manifest.hostAgents.reviewer.mode -ne 'subagent') { throw 'release manifest does not declare Reviewer host registration' }
$reviewerHash = (Get-FileHash -Algorithm SHA256 $reviewerPath).Hash.ToLowerInvariant()
$commandHash = (Get-FileHash -Algorithm SHA256 $commandPath).Hash.ToLowerInvariant()
$toolHash = (Get-FileHash -Algorithm SHA256 $toolPath).Hash.ToLowerInvariant()
if ($manifest.hostAgents.reviewer.sha256 -ne $reviewerHash -or $manifest.hostAgents.reviewer.commandSha256 -ne $commandHash -or $manifest.hostAgents.reviewer.submissionToolSha256 -ne $toolHash) { throw 'Reviewer host registration hashes do not match package bytes' }
Write-Output 'REVIEWER_HOST_REGISTRATION PASS'

$contract = Get-Content (Join-Path $fixture '.code-harness/contracts/reviewer-host-contract.md') -Raw
$bootstrap = Get-Content (Join-Path $fixture '.code-harness/bootstrap.md') -Raw
foreach ($needle in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP','Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself')) {
    if (($contract + "`n" + $bootstrap) -notmatch [regex]::Escape($needle)) { throw "missing fail-closed contract: $needle" }
}
Write-Output 'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN CONTRACT PASS'

[IO.File]::WriteAllText((Join-Path $fixture '.gitignore'), ".code-harness/`n.opencode/`nopencode.json`n", [Text.UTF8Encoding]::new($false))
$sourcePath = Join-Path $fixture 'src/main/resources/application.yml'
New-Item -ItemType Directory -Force (Split-Path -Parent $sourcePath) | Out-Null
[IO.File]::WriteAllText($sourcePath, "feature: false`n", [Text.UTF8Encoding]::new($false))
& git -C $fixture init -b develop | Out-Null
& git -C $fixture config user.email task164-e2e@example.test
& git -C $fixture config user.name 'Task164 Reviewer E2E'
& git -C $fixture add .gitignore src
& git -C $fixture commit -m 'task164 e2e base' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'failed to create E2E git baseline' }
[IO.File]::WriteAllText($sourcePath, "feature: true`n", [Text.UTF8Encoding]::new($false))

$positiveRun = 'task164-reviewer-e2e'
Initialize-AnalysisScenario $fixture $positiveRun

$port = Get-Random -Minimum 22000 -Maximum 42000
$requestLog = Join-Path $env:RUNNER_TEMP ('task164-reviewer-provider-' + [guid]::NewGuid().ToString('N') + '.jsonl')
$server = Join-Path $env:RUNNER_TEMP ('task164-reviewer-provider-' + [guid]::NewGuid().ToString('N') + '.py')
$serverCode = @'
import json, sys, time, uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1])
LOG = sys.argv[2]

def flatten(v):
    if isinstance(v, str): return v
    if isinstance(v, list): return "\n".join(flatten(x) for x in v)
    if isinstance(v, dict): return "\n".join(f"{k}:{flatten(x)}" for k, x in v.items())
    return str(v)

class H(BaseHTTPRequestHandler):
    def log_message(self, *_): pass
    def _json(self, code, obj):
        raw=json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(raw))); self.end_headers(); self.wfile.write(raw)
    def base(self, body):
        return {'id':'chatcmpl-'+uuid.uuid4().hex,'object':'chat.completion','created':int(time.time()),'model':body.get('model','reviewer-e2e')}
    def sse(self, chunks):
        self.send_response(200); self.send_header('Content-Type','text/event-stream'); self.send_header('Cache-Control','no-cache'); self.end_headers()
        for c in chunks:
            self.wfile.write(('data: '+json.dumps(c, ensure_ascii=False)+'\n\n').encode()); self.wfile.flush()
        self.wfile.write(b'data: [DONE]\n\n'); self.wfile.flush()
    def text(self, body, text):
        b=self.base(body)
        if body.get('stream'):
            self.sse([{**b,'object':'chat.completion.chunk','choices':[{'index':0,'delta':{'role':'assistant','content':text},'finish_reason':None}]},{**b,'object':'chat.completion.chunk','choices':[{'index':0,'delta':{},'finish_reason':'stop'}]}])
        else:
            self._json(200,{**b,'choices':[{'index':0,'message':{'role':'assistant','content':text},'finish_reason':'stop'}]})
    def tool(self, body, name, args):
        b=self.base(body); cid='call_'+uuid.uuid4().hex
        call={'id':cid,'type':'function','function':{'name':name,'arguments':json.dumps(args, ensure_ascii=False)}}
        if body.get('stream'):
            self.sse([{**b,'object':'chat.completion.chunk','choices':[{'index':0,'delta':{'role':'assistant','tool_calls':[{'index':0,**call}]},'finish_reason':None}]},{**b,'object':'chat.completion.chunk','choices':[{'index':0,'delta':{},'finish_reason':'tool_calls'}]}])
        else:
            self._json(200,{**b,'choices':[{'index':0,'message':{'role':'assistant','content':None,'tool_calls':[call]},'finish_reason':'tool_calls'}]})
    def do_GET(self):
        if self.path.endswith('/models'): self._json(200,{'object':'list','data':[{'id':'reviewer-e2e','object':'model','owned_by':'task164'}]})
        else: self._json(200,{'ok':True})
    def do_POST(self):
        n=int(self.headers.get('Content-Length','0')); raw=self.rfile.read(n)
        try: body=json.loads(raw)
        except Exception as e: self._json(400,{'error':str(e)}); return
        messages=body.get('messages') or []; tools=body.get('tools') or []; text=flatten(messages)
        names=[str(t.get('function',{}).get('name','')) for t in tools]
        with open(LOG,'a',encoding='utf-8') as f: f.write(json.dumps({'names':names,'messages':messages},ensure_ascii=False)+'\n')
        if not tools:
            self.text(body,'Task164 Reviewer E2E'); return
        submit=''
        for t in tools:
            fn=t.get('function',{}); desc=str(fn.get('description',''))
            if 'Submit a Codea Harness semantic proposal' in desc or ('reviewer' in str(fn.get('name','')).lower() and 'submit' in str(fn.get('name','')).lower()):
                submit=str(fn.get('name','')); break
        if any(x in text for x in ['REVIEWER_PROPOSAL_SUBMITTED','REVIEWER_MALFORMED_OUTPUT','MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN']):
            self.text(body,'TASK164_TOOL_PHASE_COMPLETE'); return
        if 'task164-reviewer-no-proposal' in text:
            self.text(body,'REVIEWER_NO_VALID_PROPOSAL'); return
        if not submit:
            self.text(body,'REVIEWER_SUBMISSION_TOOL_UNAVAILABLE'); return
        run='task164-reviewer-e2e'
        for candidate in ['task164-reviewer-malformed','task164-main-fallback','task164-reviewer-e2e']:
            if candidate in text: run=candidate; break
        if 'task164-reviewer-malformed' in text:
            self.tool(body,submit,{'kind':'change-analysis','runId':run,'proposal':'not-json'}); return
        if 'phase=FINDINGS' in text:
            self.tool(body,submit,{'kind':'findings','runId':run,'proposal':'[]'}); return
        proposal={
          'changedFileRoles':[{'path':'src/main/resources/application.yml','role':'YamlConfig'}],
          'affectedControllers':[], 'callChains':[], 'symbolLocations':[], 'resourceRelations':[],
          'externalDependencies':[], 'riskAreas':[],
          'reviewCoverage':{'status':'COMPLETE','reviewedFiles':[{'path':'src/main/resources/application.yml','role':'YamlConfig','reason':'CHANGED'}],'unresolvedSymbols':[]}
        }
        self.tool(body,submit,{'kind':'change-analysis','runId':run,'proposal':json.dumps(proposal,separators=(',',':'))})

ThreadingHTTPServer(('127.0.0.1',PORT),H).serve_forever()
'@
[IO.File]::WriteAllText($server, $serverCode, [Text.UTF8Encoding]::new($false))

$config = @{
    '$schema' = 'https://opencode.ai/config.json'
    provider = @{
        mock = @{
            npm = '@ai-sdk/openai-compatible'
            name = 'Task164 Local Mock'
            options = @{ baseURL = "http://127.0.0.1:$port/v1"; apiKey = 'task164-local' }
            models = @{ 'reviewer-e2e' = @{ name = 'Reviewer E2E'; limit = @{ context = 32000; output = 2048 } } }
        }
    }
} | ConvertTo-Json -Depth 10
[IO.File]::WriteAllText((Join-Path $fixture 'opencode.json'), $config, [Text.UTF8Encoding]::new($false))

$serverProcess = Start-Process -FilePath python -ArgumentList @($server, $port, $requestLog) -PassThru -WindowStyle Hidden
try {
    $ready = $false
    for ($i = 0; $i -lt 50; $i++) {
        try { $null = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -TimeoutSec 1; $ready = $true; break } catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $ready) { throw 'local E2E model provider did not start' }

    Push-Location $fixture
    try {
        $inventory = (& opencode agent list 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or $inventory -notmatch '(?m)^reviewer\b') { throw "Reviewer not resolvable by OpenCode host:`n$inventory" }
        Write-Output 'REVIEWER_HOST_REACHABILITY PASS'

        $input = "runId=$positiveRun phase=CHANGE_ANALYSIS snapshotPath=.code-harness/runs/$positiveRun/analysis/change-set.json proposalPath=.code-harness/runs/$positiveRun/requests/change-analysis-proposal.json. Use codea-reviewer-submit exactly once."
        $runOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json --title task164-reviewer-e2e $input 2>&1 | Out-String)
        $runExit = $LASTEXITCODE
        Write-Output $runOutput
        if ($runExit -ne 0) { throw "Reviewer OpenCode invocation failed exit=$runExit" }

        $parentMatch = [regex]::Match($runOutput, '"parentSessionId":"(?<id>ses_[^"]+)"')
        $childMatch = [regex]::Match($runOutput, '"sessionId":"(?<id>ses_[^"]+)"')
        if (-not $parentMatch.Success -or -not $childMatch.Success) { throw 'OpenCode task event did not expose parent/child session identity' }
        $parentId = $parentMatch.Groups['id'].Value
        $childId = $childMatch.Groups['id'].Value
        if ($parentId -eq $childId) { throw 'Reviewer child identity equals parent identity' }
        if ($runOutput -notmatch '"subagent_type":"reviewer"' -or $runOutput -notmatch '"command":"harness-review-reviewer"') { throw 'OpenCode task event is not bound to reviewer subagent command' }

        $proposalPath = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/change-analysis-proposal.json"
        $receiptPath = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/change-analysis-reviewer-authority.json"
        if (-not (Test-Path $proposalPath -PathType Leaf) -or -not (Test-Path $receiptPath -PathType Leaf)) { throw 'Reviewer child did not publish proposal + authority receipt through host tool' }
        $receipt = Get-Content $receiptPath -Raw | ConvertFrom-Json
        if ($receipt.agent -ne 'reviewer' -or $receipt.sessionId -ne $childId -or $receipt.proposalKind -ne 'change-analysis') { throw "Reviewer receipt identity mismatch: $($receipt | ConvertTo-Json -Compress)" }
        Write-Output "REVIEWER_INDEPENDENT_INVOCATION PASS parentSession=$parentId childSession=$childId"
        Write-Output "REVIEWER_IDENTITY_EVIDENCE PASS agent=reviewer childSession=$childId"
        Write-Output 'REVIEWER_CHANGE_ANALYSIS_PROPOSAL_HOST_RECEIPT PASS'
    } finally {
        Pop-Location
    }

    Write-AnalysisCertifyRequest $fixture $positiveRun
    $null = Invoke-Runtime $fixture @('analysis','certify','--input',".code-harness/runs/$positiveRun/requests/analysis-certify-request.json")
    if (-not (Test-Path (Join-Path $fixture ".code-harness/runs/$positiveRun/analysis/change-analysis.cert.json") -PathType Leaf)) { throw 'Runtime did not certify Reviewer ChangeAnalysis proposal' }
    Write-Output 'REVIEWER_PROPOSAL_RUNTIME_CERTIFICATION PASS'

    $optionsRequest = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/review-options-request.json"
    Write-Utf8Json $optionsRequest ([ordered]@{ runId=$positiveRun; changeAnalysisPath=".code-harness/runs/$positiveRun/analysis/change-analysis.json" })
    $null = Invoke-Runtime $fixture @('review','options','--input',".code-harness/runs/$positiveRun/requests/review-options-request.json")
    $options = Get-Content (Join-Path $fixture ".code-harness/runs/$positiveRun/analysis/review-options.json") -Raw | ConvertFrom-Json
    if ($options.decision -ne 'AUTO_FULL') { throw "expected AUTO_FULL for resource-only E2E, got $($options.decision)" }
    Write-Output 'REVIEWER_VALID_PROPOSAL_REVIEW_OPTIONS PASS decision=AUTO_FULL'

    $selectionRequest = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/review-selection-request.json"
    Write-Utf8Json $selectionRequest ([ordered]@{ runId=$positiveRun; mode='FULL'; selectionIds=@(); optionsHash=[string]$options.optionsHash })
    $null = Invoke-Runtime $fixture @('review','select','--input',".code-harness/runs/$positiveRun/requests/review-selection-request.json")
    $null = Invoke-Runtime $fixture @('review','units','--run-id',$positiveRun)
    $null = Invoke-Runtime $fixture @('review','dispatch','--run-id',$positiveRun)

    Push-Location $fixture
    try {
        $findingInput = "runId=$positiveRun phase=FINDINGS reviewUnitsPath=.code-harness/runs/$positiveRun/analysis/review-units.json proposalPath=.code-harness/runs/$positiveRun/requests/finding-proposals.json. Submit the benign empty findings array with codea-reviewer-submit."
        $findingOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json --title task164-reviewer-findings $findingInput 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Reviewer findings invocation failed:`n$findingOutput" }
        $findingChild = [regex]::Match($findingOutput, '"sessionId":"(?<id>ses_[^"]+)"')
        if (-not $findingChild.Success -or $findingOutput -notmatch '"subagent_type":"reviewer"') { throw 'finding phase did not run as independent Reviewer child' }
        $findingReceiptPath = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/finding-reviewer-authority.json"
        $findingProposalPath = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/finding-proposals.json"
        if (-not (Test-Path $findingReceiptPath -PathType Leaf) -or -not (Test-Path $findingProposalPath -PathType Leaf)) { throw 'Reviewer finding child did not publish proposal + receipt' }
        $findingReceipt = Get-Content $findingReceiptPath -Raw | ConvertFrom-Json
        $findingProposalRaw = (Get-Content $findingProposalPath -Raw).Trim()
        if ($findingReceipt.agent -ne 'reviewer' -or $findingReceipt.sessionId -ne $findingChild.Groups['id'].Value -or $findingReceipt.proposalKind -ne 'findings') { throw 'finding receipt identity mismatch' }
        if ($findingProposalRaw -ne '[]') { throw "finding E2E must submit an empty JSON array, got: $findingProposalRaw" }
        Write-Output "REVIEWER_FINDING_PROPOSAL_HOST_RECEIPT PASS childSession=$($findingChild.Groups['id'].Value)"
    } finally {
        Pop-Location
    }

    $findingCertify = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/finding-certify-request.json"
    Write-Utf8Json $findingCertify ([ordered]@{ runId=$positiveRun; proposalsPath=".code-harness/runs/$positiveRun/requests/finding-proposals.json" })
    $null = Invoke-Runtime $fixture @('review','certify-findings','--input',".code-harness/runs/$positiveRun/requests/finding-certify-request.json")
    if (-not (Test-Path (Join-Path $fixture ".code-harness/runs/$positiveRun/analysis/certified-findings.cert.json") -PathType Leaf)) { throw 'Runtime did not certify Reviewer findings' }
    Write-Output 'REVIEWER_FINDING_RUNTIME_CERTIFICATION PASS'

    $analysis = Get-Content (Join-Path $fixture ".code-harness/runs/$positiveRun/analysis/change-analysis.json") -Raw | ConvertFrom-Json
    $reportRequest = Join-Path $fixture ".code-harness/runs/$positiveRun/requests/review-report.json"
    Write-Utf8Json $reportRequest ([ordered]@{
        runId=$positiveRun; harnessVersion='transport-not-authority'; baseRef=[string]$analysis.reviewScope.baseRef; head=[string]$analysis.reviewScope.headCommit
        result='FAILED'; mode='FULL'
        reviewScope=[ordered]@{ changedFiles=@('src/main/resources/application.yml') }
        reviewCoverage=[ordered]@{ reviewedFiles=@('src/main/resources/application.yml'); callChains=@(); externalDependencies=@(); unresolved=@(); missingReviewedFiles=@(); runtimeErrors=@(); status='COMPLETE' }
        findings=@()
    })
    $null = Invoke-Runtime $fixture @('report','review','--input',".code-harness/runs/$positiveRun/requests/review-report.json")
    $report = Get-Content (Join-Path $fixture ".code-harness/runs/$positiveRun/review.md") -Raw
    $passedLabel = '| 评审结果 | ✅ 通过 |'
    if ($report -notmatch [regex]::Escape($passedLabel)) { throw "Runtime-certified empty Reviewer findings did not drive canonical PASSED report:`n$report" }
    Write-Output 'REVIEWER_RUNTIME_AUTHORITY_SEPARATION PASS'

    $negative = Join-Path $env:RUNNER_TEMP ('task164-reviewer-missing-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $negative
    Remove-Item (Join-Path $negative '.opencode/agents/reviewer.md') -Force
    $missingRun = 'task164-reviewer-unavailable'
    Initialize-AnalysisScenario $negative $missingRun
    Push-Location $negative
    try {
        $negativeInventory = (& opencode agent list 2>&1 | Out-String)
        if ($LASTEXITCODE -eq 0 -and $negativeInventory -match '(?m)^reviewer\b') { throw 'Reviewer remained host-resolvable after registration removal' }
        $negativeOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json "runId=$missingRun phase=CHANGE_ANALYSIS" 2>&1 | Out-String)
        if ($negativeOutput -match '"subagent_type":"reviewer"') { throw 'OpenCode spawned Reviewer after registration was removed' }
    } finally { Pop-Location }
    $null = Assert-AnalysisHardStop $negative $missingRun
    Write-Output 'REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS'

    $nonInvokable = Join-Path $env:RUNNER_TEMP ('task164-reviewer-noninvokable-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $nonInvokable
    $nonInvokableAgent = Join-Path $nonInvokable '.opencode/agents/reviewer.md'
    $badAgent = (Get-Content $nonInvokableAgent -Raw).Replace('mode: subagent','mode: definitely-invalid')
    [IO.File]::WriteAllText($nonInvokableAgent, $badAgent, [Text.UTF8Encoding]::new($false))
    Push-Location $nonInvokable
    try {
        $badInventory = (& opencode agent list 2>&1 | Out-String)
        if ($LASTEXITCODE -eq 0 -and $badInventory -match '(?m)^reviewer\b') { throw 'invalid Reviewer host mode remained invokable' }
    } finally { Pop-Location }
    Write-Output 'REVIEWER_FILE_PRESENT_NOT_HOST_INVOKABLE_FAIL_CLOSED PASS'

    $invokeFail = Join-Path $env:RUNNER_TEMP ('task164-reviewer-invoke-fail-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $invokeFail
    $invokeFailRun = 'task164-reviewer-invocation-failure'
    Initialize-AnalysisScenario $invokeFail $invokeFailRun
    Push-Location $invokeFail
    try {
        $invokeFailOutput = (& opencode run --command harness-review-reviewer --model mock/model-does-not-exist --format json "runId=$invokeFailRun phase=CHANGE_ANALYSIS" 2>&1 | Out-String)
        $invokeFailExit = $LASTEXITCODE
    } finally { Pop-Location }
    if ($invokeFailExit -eq 0) { throw "missing-model Reviewer invocation unexpectedly succeeded:`n$invokeFailOutput" }
    $null = Assert-AnalysisHardStop $invokeFail $invokeFailRun
    Write-Output 'REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED PASS'

    $malformed = Join-Path $env:RUNNER_TEMP ('task164-reviewer-malformed-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $malformed
    $malformedRun = 'task164-reviewer-malformed'
    Initialize-AnalysisScenario $malformed $malformedRun
    Push-Location $malformed
    try {
        $malformedOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json "runId=$malformedRun phase=CHANGE_ANALYSIS submit malformed payload" 2>&1 | Out-String)
    } finally { Pop-Location }
    if ($malformedOutput -notmatch '"subagent_type":"reviewer"') { throw "malformed case did not execute an independent Reviewer child:`n$malformedOutput" }
    $providerTranscript = if (Test-Path $requestLog -PathType Leaf) { Get-Content $requestLog -Raw } else { '' }
    if ($providerTranscript -notmatch 'REVIEWER_MALFORMED_OUTPUT') { throw "real Reviewer malformed tool rejection marker missing from child provider transcript:`n$malformedOutput" }
    if (Test-Path (Join-Path $malformed ".code-harness/runs/$malformedRun/requests/change-analysis-reviewer-authority.json")) { throw 'malformed Reviewer output published authority receipt' }
    $null = Assert-AnalysisHardStop $malformed $malformedRun
    Write-Output 'REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED PASS'

    $noProposal = Join-Path $env:RUNNER_TEMP ('task164-reviewer-no-proposal-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $noProposal
    $noProposalRun = 'task164-reviewer-no-proposal'
    Initialize-AnalysisScenario $noProposal $noProposalRun
    Push-Location $noProposal
    try {
        $noProposalOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json "runId=$noProposalRun phase=CHANGE_ANALYSIS complete without submission" 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "no-proposal child should complete normally so Runtime can reject missing authority:`n$noProposalOutput" }
    } finally { Pop-Location }
    if (Test-Path (Join-Path $noProposal ".code-harness/runs/$noProposalRun/requests/change-analysis-reviewer-authority.json")) { throw 'no-proposal child unexpectedly published receipt' }
    $null = Assert-AnalysisHardStop $noProposal $noProposalRun
    Write-Output 'REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED PASS'

    $mainFallback = Join-Path $env:RUNNER_TEMP ('task164-main-fallback-' + [guid]::NewGuid().ToString('N'))
    Copy-Item -Recurse -Force $fixture $mainFallback
    $mainRun = 'task164-main-fallback'
    Initialize-AnalysisScenario $mainFallback $mainRun
    Push-Location $mainFallback
    try {
        $mainOutput = (& opencode run --model mock/reviewer-e2e --format json "runId=$mainRun phase=MAIN_AGENT_FALLBACK submit semantic proposal with codea-reviewer-submit" 2>&1 | Out-String)
    } finally { Pop-Location }
    if ($mainOutput -notmatch 'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN') { throw "Main Agent submission was not rejected by Host identity gate:`n$mainOutput" }
    if (Test-Path (Join-Path $mainFallback ".code-harness/runs/$mainRun/requests/change-analysis-reviewer-authority.json")) { throw 'Main Agent fallback published Reviewer authority receipt' }
    $null = Assert-AnalysisHardStop $mainFallback $mainRun
    Write-Output 'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS'

    Write-Output 'gate_task2_product_e2e PASS'
    Write-Output 'TASK164_RELEASE_BLOCKER_TASK2_E2E PASS'
} finally {
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    Remove-Item $server -Force -ErrorAction SilentlyContinue
}
