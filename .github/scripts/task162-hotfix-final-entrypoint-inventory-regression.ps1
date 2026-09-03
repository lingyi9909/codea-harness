$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness/bin/codea-dcep-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness/bin/ast-grep.exe'
$schemaSource = Join-Path $repoRoot '.code-harness/contracts/entrypoint-inventory.schema.json'
$requestSchemaSource = Join-Path $repoRoot '.code-harness/contracts/analysis-inventory-request.schema.json'

foreach ($required in @($runtimeSource, $astGrepSource, $schemaSource, $requestSchemaSource)) {
    if (-not (Test-Path $required -PathType Leaf)) {
        throw "Final EntryPoint compatibility regression missing required file: $required"
    }
}

$fixture = Join-Path $env:RUNNER_TEMP ("task162-final-entrypoint-inventory-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    New-Item -ItemType Directory -Force (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

try {
    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task162-final@example.test'
        Invoke-Git config user.name 'Task 162 Final EntryPoint Regression'

        $cPath = Join-Path $fixture 'src/main/java/acme/CController.java'
        $fakePath = Join-Path $fixture 'src/main/java/acme/FakeController.java'

        Write-Utf8NoBom $cPath @'
package acme;

@RestController
public class CController {
    @PutMapping
    public void update() {
        int value = 1;
        System.out.println(value);
    }
}
'@
        Write-Utf8NoBom $fakePath @'
package acme;

public class FakeController {
    public void fake() {
        int value = 1;
        System.out.println(value);
    }
}
'@

        Invoke-Git add 'src/main/java/acme/CController.java' 'src/main/java/acme/FakeController.java'
        Invoke-Git commit -m 'base controllers'

        $aPath = Join-Path $fixture 'src/main/java/acme/AController.java'
        $bPath = Join-Path $fixture 'src/main/java/acme/BController.java'

        Write-Utf8NoBom $aPath @'
package acme;

@RestController
public class AController {
    @PostMapping
    public void create() {
    }
}
'@
        Invoke-Git add 'src/main/java/acme/AController.java'

        Write-Utf8NoBom $bPath @'
package acme;

@Controller
public class BController {
    @PostMapping
    public void submit() {
    }
}
'@

        Write-Utf8NoBom $cPath @'
package acme;

@RestController
public class CController {
    @PutMapping
    public void update() {
        int value = 2;
        System.out.println(value);
    }
}
'@
        Write-Utf8NoBom $fakePath @'
package acme;

public class FakeController {
    public void fake() {
        int value = 2;
        System.out.println(value);
    }
}
'@

        $status = @(& git status --porcelain=v1)
        if ($LASTEXITCODE -ne 0) { throw 'git status failed' }
        if (-not ($status -contains 'A  src/main/java/acme/AController.java')) { throw "AController is not staged: $($status -join '; ')" }
        if (-not ($status -contains '?? src/main/java/acme/BController.java')) { throw "BController is not untracked: $($status -join '; ')" }
        if (-not ($status -contains ' M src/main/java/acme/CController.java')) { throw "CController is not unstaged: $($status -join '; ')" }
        if (-not ($status -contains ' M src/main/java/acme/FakeController.java')) { throw "FakeController is not unstaged: $($status -join '; ')" }
        Write-Host 'TASK153 REAL CHANGESET STAGED + UNTRACKED + UNSTAGED PASS'

        New-Item -ItemType Directory -Force '.code-harness/bin' | Out-Null
        New-Item -ItemType Directory -Force '.code-harness/contracts' | Out-Null
        Copy-Item $runtimeSource '.code-harness/bin/codea-dcep-tools.exe' -Force
        Copy-Item $astGrepSource '.code-harness/bin/ast-grep.exe' -Force
        Copy-Item $schemaSource '.code-harness/contracts/entrypoint-inventory.schema.json' -Force
        Copy-Item $requestSchemaSource '.code-harness/contracts/analysis-inventory-request.schema.json' -Force

        New-Item -ItemType Directory -Force '.code-harness/runs/task153-real/requests' | Out-Null
        Write-Utf8NoBom (Join-Path $fixture '.code-harness/runs/task153-real/requests/entrypoint-inventory.json') '{"runId":"task153-real","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"}}'

        & '.code-harness/bin/codea-dcep-tools.exe' analysis inventory --input '.code-harness/runs/task153-real/requests/entrypoint-inventory.json'
        if ($LASTEXITCODE -ne 0) { throw "analysis inventory failed with exit code $LASTEXITCODE" }

        $artifactPath = Join-Path $fixture '.code-harness/runs/task153-real/analysis/entrypoint-inventory.json'
        if (-not (Test-Path $artifactPath -PathType Leaf)) { throw 'analysis inventory artifact missing' }
        $artifact = Get-Content $artifactPath -Raw | ConvertFrom-Json
        $entrypoints = @($artifact.expectedEntryPoints)
        $symbols = @($entrypoints | ForEach-Object { [string]$_.symbol } | Sort-Object)
        $expected = @('AController.create', 'BController.submit', 'CController.update')

        if ($entrypoints.Count -ne 3) {
            throw "expected exactly 3 entrypoints, got $($entrypoints.Count): $($symbols -join ', ')"
        }
        if (($symbols -join '|') -ne ($expected -join '|')) {
            throw "unexpected entrypoints: $($symbols -join ', ')"
        }
        if ($symbols -contains 'FakeController.fake' -or ($symbols | Where-Object { $_ -like 'FakeController.*' })) {
            throw "FakeController leaked into expectedEntryPoints: $($symbols -join ', ')"
        }

        Write-Host 'TASK153 REAL AST_GREP ENTRYPOINTS AController.create + BController.submit + CController.update EXACTLY 3 PASS'
        Write-Host 'TASK153 FakeController excluded PASS'
        Write-Output 'TASK153_TASK1_REAL_ENTRYPOINT_INVENTORY PASS'
        Write-Output 'TASK162_FINAL_ENTRYPOINT_TASK2_REQUEST_CONTRACT_COMPAT PASS'
    }
    finally {
        Pop-Location
    }
}
finally {
    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
}
