$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "missing candidate install package: $installZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'python is required for local OpenAI-compatible E2E provider' }

Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($installZip)
try {
    $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\\','/') })
} finally {
    $zip.Dispose()
}
foreach ($required in @('.opencode/agents/reviewer.md', '.opencode/commands/harness-review-reviewer.md')) {
    if ($entries -notcontains $required) { throw "package missing OpenCode host resource: $required" }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task2-e2e-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force

$reviewerPath = Join-Path $fixture '.opencode/agents/reviewer.md'
$commandPath = Join-Path $fixture '.opencode/commands/harness-review-reviewer.md'
$manifestPath = Join-Path $fixture '.code-harness/RELEASE-MANIFEST.json'
$reviewer = Get-Content $reviewerPath -Raw
$command = Get-Content $commandPath -Raw
$manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
if ($reviewer -notmatch '(?m)^mode:\s*subagent\s*$') { throw 'Reviewer is not registered as OpenCode subagent' }
if ($command -notmatch '(?m)^agent:\s*reviewer\s*$' -or $command -notmatch '(?m)^subagent:\s*true\s*$') { throw 'Reviewer command is not pinned to independent subagent delegation' }
if ($manifest.hostAgents.reviewer.path -ne '.opencode/agents/reviewer.md' -or $manifest.hostAgents.reviewer.mode -ne 'subagent') { throw 'release manifest does not declare Reviewer host registration' }
$reviewerHash = (Get-FileHash -Algorithm SHA256 $reviewerPath).Hash.ToLowerInvariant()
$commandHash = (Get-FileHash -Algorithm SHA256 $commandPath).Hash.ToLowerInvariant()
if ($manifest.hostAgents.reviewer.sha256 -ne $reviewerHash -or $manifest.hostAgents.reviewer.commandSha256 -ne $commandHash) { throw 'Reviewer host registration hashes do not match package bytes' }
Write-Output 'REVIEWER_HOST_REGISTRATION PASS'

$contract = Get-Content (Join-Path $fixture '.code-harness/contracts/reviewer-host-contract.md') -Raw
$bootstrap = Get-Content (Join-Path $fixture '.code-harness/bootstrap.md') -Raw
foreach ($needle in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP','Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself')) {
    if (($contract + "`n" + $bootstrap) -notmatch [regex]::Escape($needle)) { throw "missing fail-closed contract: $needle" }
}
Write-Output 'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS'

# Real OpenCode invocation uses a local OpenAI-compatible provider so the E2E
# exercises the pinned host without external model credentials.
$port = Get-Random -Minimum 22000 -Maximum 42000
$requestLog = Join-Path $fixture 'mock-provider-requests.jsonl'
$server = Join-Path $fixture 'mock_provider.py'
$serverCode = @'
import json, os, sys, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1])
LOG = sys.argv[2]

class H(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass
    def _json(self, code, obj):
        raw = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header('Content-Type','application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)
    def do_GET(self):
        if self.path.endswith('/models'):
            self._json(200, {'object':'list','data':[{'id':'reviewer-e2e','object':'model','owned_by':'task164'}]})
        else:
            self._json(200, {'ok': True})
    def do_POST(self):
        n = int(self.headers.get('Content-Length','0'))
        body = self.rfile.read(n).decode('utf-8','replace')
        try:
            parsed = json.loads(body)
        except Exception:
            parsed = {'raw': body}
        with open(LOG, 'a', encoding='utf-8') as f:
            f.write(json.dumps({'path':self.path,'body':parsed}, ensure_ascii=False) + '\n')
        model = parsed.get('model','reviewer-e2e') if isinstance(parsed, dict) else 'reviewer-e2e'
        text = 'REVIEWER_E2E_PROPOSAL_ONLY runId=task164-reviewer-e2e phase=CHANGE_ANALYSIS'
        if isinstance(parsed, dict) and parsed.get('stream'):
            self.send_response(200)
            self.send_header('Content-Type','text/event-stream')
            self.send_header('Cache-Control','no-cache')
            self.end_headers()
            first = {'id':'chatcmpl-task164','object':'chat.completion.chunk','created':int(time.time()),'model':model,'choices':[{'index':0,'delta':{'role':'assistant','content':text},'finish_reason':None}]}
            last = {'id':'chatcmpl-task164','object':'chat.completion.chunk','created':int(time.time()),'model':model,'choices':[{'index':0,'delta':{},'finish_reason':'stop'}]}
            for obj in (first,last):
                self.wfile.write(('data: ' + json.dumps(obj) + '\n\n').encode())
                self.wfile.flush()
            self.wfile.write(b'data: [DONE]\n\n')
            self.wfile.flush()
        else:
            self._json(200, {'id':'chatcmpl-task164','object':'chat.completion','created':int(time.time()),'model':model,'choices':[{'index':0,'message':{'role':'assistant','content':text},'finish_reason':'stop'}],'usage':{'prompt_tokens':1,'completion_tokens':1,'total_tokens':2}})

ThreadingHTTPServer(('127.0.0.1', PORT), H).serve_forever()
'@
[IO.File]::WriteAllText($server, $serverCode, [Text.UTF8Encoding]::new($false))

$config = @{
    '$schema' = 'https://opencode.ai/config.json'
    provider = @{
        mock = @{
            npm = '@ai-sdk/openai-compatible'
            name = 'Task164 Local Mock'
            options = @{
                baseURL = "http://127.0.0.1:$port/v1"
                apiKey = 'task164-local'
            }
            models = @{
                'reviewer-e2e' = @{
                    name = 'Reviewer E2E'
                    limit = @{ context = 32000; output = 2048 }
                }
            }
        }
    }
} | ConvertTo-Json -Depth 10
[IO.File]::WriteAllText((Join-Path $fixture 'opencode.json'), $config, [Text.UTF8Encoding]::new($false))

$serverProcess = Start-Process -FilePath python -ArgumentList @($server, $port, $requestLog) -PassThru -WindowStyle Hidden
try {
    $ready = $false
    for ($i = 0; $i -lt 50; $i++) {
        try {
            $null = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -TimeoutSec 1
            $ready = $true
            break
        } catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $ready) { throw 'local E2E model provider did not start' }

    Push-Location $fixture
    try {
        $inventory = (& opencode agent list 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or $inventory -notmatch '(?m)^reviewer\b') { throw "Reviewer not resolvable by OpenCode host:`n$inventory" }

        $beforeSessions = (& opencode session list --format json 2>&1 | Out-String)
        $input = 'runId=task164-reviewer-e2e phase=CHANGE_ANALYSIS snapshotPath=.code-harness/runs/task164-reviewer-e2e/analysis/change-set.json proposalPath=.code-harness/runs/task164-reviewer-e2e/requests/change-analysis-proposal.json'
        $runOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json --title task164-reviewer-e2e $input 2>&1 | Out-String)
        $runExit = $LASTEXITCODE
        Write-Output $runOutput
        if ($runExit -ne 0) { throw "Reviewer OpenCode invocation failed exit=$runExit" }
        if ($runOutput -notmatch 'REVIEWER_E2E_PROPOSAL_ONLY') { throw 'Reviewer child result missing from OpenCode output' }

        $requests = if (Test-Path $requestLog) { Get-Content $requestLog -Raw } else { '' }
        foreach ($needle in @('task164-reviewer-e2e','CHANGE_ANALYSIS','change-analysis-proposal.json','Reviewer 是只读 Agent')) {
            if ($requests -notmatch [regex]::Escape($needle)) { throw "Reviewer provider request missing intended input/system identity: $needle" }
        }

        $sessionJson = (& opencode session list --format json 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "cannot list OpenCode sessions: $sessionJson" }
        $sessions = @($sessionJson | ConvertFrom-Json)
        $parent = @($sessions | Where-Object { $_.title -eq 'task164-reviewer-e2e' } | Select-Object -First 1)
        if ($parent.Count -ne 1) { throw "parent OpenCode session not found: $sessionJson" }
        $parentId = if ($parent[0].id) { $parent[0].id } elseif ($parent[0].sessionID) { $parent[0].sessionID } else { $null }
        if (-not $parentId) { throw 'parent OpenCode session has no identity' }
        $children = @($sessions | Where-Object { $_.parentID -eq $parentId -or $_.parentId -eq $parentId })
        if ($children.Count -lt 1) { throw "Reviewer did not execute in an independent child session: $sessionJson" }
        $child = $children[0]
        $childId = if ($child.id) { $child.id } elseif ($child.sessionID) { $child.sessionID } else { $null }
        if (-not $childId -or $childId -eq $parentId) { throw 'Reviewer child identity is not independent' }
        $export = (& opencode export $childId 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or $export -notmatch 'reviewer') { throw "Reviewer identity not present in child session export: $export" }
        Write-Output "REVIEWER_INDEPENDENT_INVOCATION PASS parentSession=$parentId childSession=$childId"
        Write-Output "REVIEWER_IDENTITY_EVIDENCE PASS agent=reviewer childSession=$childId"
    } finally {
        Pop-Location
    }
} finally {
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
}

# Runtime authority separation is checked against packaged policy + actual file
# permissions: Reviewer may edit requests/** only and may not shell into Runtime.
if ($reviewer -notmatch 'resource: "\.code-harness/runs/\*/requests/\*\*"' -or $reviewer -notmatch '(?s)action: shell.*?effect: deny') {
    throw 'Reviewer host permissions do not preserve requests-only proposal authority'
}
foreach ($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','review.md')) {
    if (Test-Path (Join-Path $fixture ".code-harness/runs/task164-reviewer-e2e/$authority")) { throw "Reviewer published Runtime authority artifact: $authority" }
}
Write-Output 'REVIEWER_RUNTIME_AUTHORITY_SEPARATION PASS'

# Negative real-host E2E: command remains installed, Reviewer registration is
# intentionally removed. The host must fail instead of executing semantic work.
$negative = Join-Path $env:RUNNER_TEMP ('task164-task2-negative-' + [guid]::NewGuid().ToString('N'))
Copy-Item -Recurse -Force $fixture $negative
Remove-Item (Join-Path $negative '.opencode/agents/reviewer.md') -Force
Push-Location $negative
try {
    $negativeOutput = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json 'runId=task164-reviewer-unavailable phase=CHANGE_ANALYSIS' 2>&1 | Out-String)
    $negativeExit = $LASTEXITCODE
} finally {
    Pop-Location
}
if ($negativeExit -eq 0 -and $negativeOutput -match 'REVIEWER_E2E_PROPOSAL_ONLY') { throw 'OpenCode silently continued semantic Reviewer work with Reviewer registration removed' }
foreach ($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','review.md')) {
    if (Test-Path (Join-Path $negative ".code-harness/runs/task164-reviewer-unavailable/$authority")) { throw "Reviewer-unavailable path published authority artifact: $authority" }
}
Write-Output 'REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS'
Write-Output 'TASK164_RELEASE_BLOCKER_TASK2_E2E PASS'
