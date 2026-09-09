$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "missing candidate install package: $installZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'python is required' }
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw 'git is required' }

function Write-Utf8Json([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 30), [Text.UTF8Encoding]::new($false))
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-reviewer-cancel-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force

[IO.File]::WriteAllText((Join-Path $fixture '.gitignore'), ".code-harness/`n.opencode/`nopencode.json`n", [Text.UTF8Encoding]::new($false))
$sourcePath = Join-Path $fixture 'src/main/resources/application.yml'
New-Item -ItemType Directory -Force (Split-Path -Parent $sourcePath) | Out-Null
[IO.File]::WriteAllText($sourcePath, "cancelCase: false`n", [Text.UTF8Encoding]::new($false))
& git -C $fixture init -b develop | Out-Null
& git -C $fixture config user.email task164-cancel@example.test
& git -C $fixture config user.name 'Task164 Reviewer Cancel E2E'
& git -C $fixture add .gitignore src
& git -C $fixture commit -m 'cancel e2e base' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'failed to create cancellation E2E baseline' }
[IO.File]::WriteAllText($sourcePath, "cancelCase: true`n", [Text.UTF8Encoding]::new($false))

$runID = 'task164-reviewer-session-cancel'
$requestRel = ".code-harness/runs/$runID/requests/change-set-request.json"
Write-Utf8Json (Join-Path $fixture $requestRel) ([ordered]@{runId=$runID;baseRef='HEAD';includeWorkingTree=$true})
$runtime = Join-Path $fixture '.code-harness/bin/codea-dcep-tools.exe'
Push-Location $fixture
try {
    $snapshotOutput = (& $runtime analysis snapshot --input $requestRel 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) { throw "snapshot failed:`n$snapshotOutput" }
} finally { Pop-Location }

$port = Get-Random -Minimum 22000 -Maximum 42000
$requestLog = Join-Path $env:RUNNER_TEMP ('task164-reviewer-cancel-provider-' + [guid]::NewGuid().ToString('N') + '.jsonl')
$serverPath = Join-Path $env:RUNNER_TEMP ('task164-reviewer-cancel-provider-' + [guid]::NewGuid().ToString('N') + '.py')
$serverCode = @'
import json, sys, time, uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT=int(sys.argv[1]); LOG=sys.argv[2]; RUN='task164-reviewer-session-cancel'

def flatten(v):
    if isinstance(v,str): return v
    if isinstance(v,list): return '\n'.join(flatten(x) for x in v)
    if isinstance(v,dict): return '\n'.join(f'{k}:{flatten(x)}' for k,x in v.items())
    return str(v)

class H(BaseHTTPRequestHandler):
    def log_message(self,*_): pass
    def send_json(self,code,obj):
        raw=json.dumps(obj,ensure_ascii=False).encode(); self.send_response(code); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(raw))); self.end_headers(); self.wfile.write(raw)
    def stream(self,body,obj):
        base={'id':'chatcmpl-'+uuid.uuid4().hex,'object':'chat.completion.chunk','created':int(time.time()),'model':body.get('model','reviewer-e2e')}
        self.send_response(200); self.send_header('Content-Type','text/event-stream'); self.end_headers()
        self.wfile.write(('data: '+json.dumps({**base,'choices':[{'index':0,'delta':obj,'finish_reason':None}]})+'\n\n').encode()); self.wfile.flush()
        self.wfile.write(('data: '+json.dumps({**base,'choices':[{'index':0,'delta':{},'finish_reason':'tool_calls' if 'tool_calls' in obj else 'stop'}]})+'\n\n').encode()); self.wfile.write(b'data: [DONE]\n\n'); self.wfile.flush()
    def do_GET(self):
        if self.path.endswith('/models'): self.send_json(200,{'object':'list','data':[{'id':'reviewer-e2e','object':'model','owned_by':'task164'}]})
        else: self.send_json(200,{'ok':True})
    def do_POST(self):
        n=int(self.headers.get('Content-Length','0')); body=json.loads(self.rfile.read(n)); msgs=body.get('messages') or []; tools=body.get('tools') or []; text=flatten(msgs)
        names=[str(t.get('function',{}).get('name','')) for t in tools]
        is_reviewer=('Reviewer 是只读 Agent' in text or '只读 Agent' in text) and RUN in text
        with open(LOG,'a',encoding='utf-8') as f: f.write(json.dumps({'reviewer':is_reviewer,'names':names,'text':text},ensure_ascii=False)+'\n')
        if is_reviewer:
            # Keep the child model request alive. The test kills the OpenCode host
            # only after this Reviewer-specific request proves the child session started.
            time.sleep(60)
            self.send_json(500,{'error':'Reviewer child should have been cancelled by test'})
            return
        if 'task' in names and RUN in text:
            args={'prompt':f'runId={RUN} phase=CHANGE_ANALYSIS. Start Reviewer semantic analysis but do not let Main Agent replace it.','description':'Task164 Reviewer cancellation E2E','subagent_type':'reviewer','command':'harness-review-reviewer'}
            call={'id':'call_'+uuid.uuid4().hex,'type':'function','function':{'name':'task','arguments':json.dumps(args)}}
            if body.get('stream'):
                self.stream(body,{'role':'assistant','tool_calls':[{'index':0,**call}]})
            else:
                base={'id':'chatcmpl-'+uuid.uuid4().hex,'object':'chat.completion','created':int(time.time()),'model':body.get('model','reviewer-e2e')}
                self.send_json(200,{**base,'choices':[{'index':0,'message':{'role':'assistant','content':None,'tool_calls':[call]},'finish_reason':'tool_calls'}]})
            return
        if body.get('stream'):
            self.stream(body,{'role':'assistant','content':'TASK164_CANCEL_PARENT_WAIT'})
        else:
            base={'id':'chatcmpl-'+uuid.uuid4().hex,'object':'chat.completion','created':int(time.time()),'model':body.get('model','reviewer-e2e')}
            self.send_json(200,{**base,'choices':[{'index':0,'message':{'role':'assistant','content':'TASK164_CANCEL_PARENT_WAIT'},'finish_reason':'stop'}]})

ThreadingHTTPServer(('127.0.0.1',PORT),H).serve_forever()
'@
[IO.File]::WriteAllText($serverPath, $serverCode, [Text.UTF8Encoding]::new($false))
$config = @{
    '$schema'='https://opencode.ai/config.json'
    provider=@{mock=@{npm='@ai-sdk/openai-compatible';name='Task164 Cancel Mock';options=@{baseURL="http://127.0.0.1:$port/v1";apiKey='task164-local'};models=@{'reviewer-e2e'=@{name='Reviewer Cancel E2E';limit=@{context=32000;output=2048}}}}}
} | ConvertTo-Json -Depth 10
[IO.File]::WriteAllText((Join-Path $fixture 'opencode.json'), $config, [Text.UTF8Encoding]::new($false))

$serverProcess = Start-Process -FilePath python -ArgumentList @($serverPath,$port,$requestLog) -PassThru -WindowStyle Hidden
$opencodeProcess = $null
try {
    $ready=$false
    for($i=0;$i -lt 50;$i++) { try { $null=Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -TimeoutSec 1; $ready=$true; break } catch { Start-Sleep -Milliseconds 100 } }
    if(-not $ready){throw 'cancel E2E provider did not start'}

    $stdout=Join-Path $env:RUNNER_TEMP ('task164-cancel-stdout-'+[guid]::NewGuid().ToString('N')+'.log')
    $stderr=Join-Path $env:RUNNER_TEMP ('task164-cancel-stderr-'+[guid]::NewGuid().ToString('N')+'.log')
    $opencodeCmd=(Get-Command opencode).Source
    $opencodeProcess=Start-Process -FilePath $opencodeCmd -ArgumentList @('run','--model','mock/reviewer-e2e','--format','json',"runId=$runID delegate semantic analysis to reviewer and wait for completion") -WorkingDirectory $fixture -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru

    $childStarted=$false
    for($i=0;$i -lt 300;$i++) {
        if(Test-Path $requestLog) {
            $log=Get-Content $requestLog -Raw
            if($log -match '"reviewer": true' -and $log -match [regex]::Escape($runID)) { $childStarted=$true; break }
        }
        if($opencodeProcess.HasExited) { break }
        Start-Sleep -Milliseconds 200
    }
    if(-not $childStarted) {
        $out=if(Test-Path $stdout){Get-Content $stdout -Raw}else{''}; $err=if(Test-Path $stderr){Get-Content $stderr -Raw}else{''}
        throw "Reviewer child never reached its model request before cancellation:`n$out`n$err"
    }
    Write-Output 'REVIEWER_SESSION_STARTED_BEFORE_CANCEL PASS'
    Stop-Process -Id $opencodeProcess.Id -Force
    $opencodeProcess.WaitForExit()
    Write-Output 'REVIEWER_SESSION_CANCELLED PASS'

    foreach($file in @('change-analysis-proposal.json','change-analysis-reviewer-authority.json')) {
        if(Test-Path (Join-Path $fixture ".code-harness/runs/$runID/requests/$file")) { throw "cancelled Reviewer published $file" }
    }

    $snapshot=Get-Content (Join-Path $fixture ".code-harness/runs/$runID/analysis/change-set.json") -Raw | ConvertFrom-Json
    $certifyRel=".code-harness/runs/$runID/requests/analysis-certify-request.json"
    Write-Utf8Json (Join-Path $fixture $certifyRel) ([ordered]@{runId=$runID;snapshotPath=".code-harness/runs/$runID/analysis/change-set.json";snapshotSha256=[string]$snapshot.snapshotSha256;proposalPath=".code-harness/runs/$runID/requests/change-analysis-proposal.json";intent=[ordered]@{mode='FULL'}})
    Push-Location $fixture
    try {
        $stopOutput=(& $runtime analysis certify --input $certifyRel 2>&1 | Out-String); $stopExit=$LASTEXITCODE
    } finally { Pop-Location }
    if($stopExit -eq 0){throw 'cancelled Reviewer incorrectly allowed analysis certification'}
    foreach($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) { if($stopOutput -notmatch [regex]::Escape($marker)){throw "cancel hard stop missing $marker`n$stopOutput"} }
    foreach($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json','analysis/review-units.json','analysis/rule-dispatch.json','analysis/certified-findings.json','review.md')) {
        if(Test-Path (Join-Path $fixture ".code-harness/runs/$runID/$authority")){throw "cancelled Reviewer path published authority: $authority"}
    }
    Write-Output 'REVIEWER_SESSION_CANCEL_FAIL_CLOSED PASS'
} finally {
    if($opencodeProcess -and -not $opencodeProcess.HasExited){Stop-Process -Id $opencodeProcess.Id -Force -ErrorAction SilentlyContinue}
    if($serverProcess -and -not $serverProcess.HasExited){Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue}
    Remove-Item $serverPath -Force -ErrorAction SilentlyContinue
}
