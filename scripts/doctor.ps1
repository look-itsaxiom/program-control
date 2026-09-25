# Reports whether the toolchain needed for program-control is installed. Needs nothing but Windows PowerShell.
$ErrorActionPreference = 'Continue'
$rows = @()
function Check($name, $required, $cmd, $minVersion, $url, [scriptblock]$parse) {
  $exe = Get-Command $cmd -ErrorAction SilentlyContinue
  if (-not $exe) { $script:rows += [pscustomobject]@{ Tool=$name; Status='MISSING'; Version=''; Need=$minVersion; Get=$url; Required=$required }; return }
  try { $v = & $parse } catch { $v = '' }
  $ok = $true
  if ($minVersion -and $v) { $ok = ([version]($v -replace '[^0-9.].*$','' -replace '\.+$','') -ge [version]$minVersion) }
  $script:rows += [pscustomobject]@{ Tool=$name; Status=$(if ($ok) {'ok'} else {'TOO OLD'}); Version=$v; Need=$minVersion; Get=$url; Required=$required }
}
Check 'git'    $true 'git'    '2.30' 'https://git-scm.com/downloads'            { (git --version) -replace 'git version ','' }
Check 'go'     $true 'go'     '1.26' 'https://go.dev/dl/'                        { (go version) -replace '^go version go','' -replace ' .*$','' }
Check 'node'   $true 'node'   '22.0' 'https://nodejs.org/'                       { (node --version) -replace '^v','' }
Check 'pnpm'   $true 'pnpm'   '10.0' 'https://pnpm.io/installation'              { pnpm --version }
Check 'docker' $true 'docker' '24.0' 'https://www.docker.com/products/docker-desktop/' { (docker version --format '{{.Client.Version}}') }
Check 'gh'     $false 'gh'    '2.40' 'https://cli.github.com/'                   { (gh --version | Select-Object -First 1) -replace '^gh version ','' -replace ' .*$','' }
Check 'claude' $false 'claude' ''    'https://docs.claude.com/en/docs/claude-code/overview' { (claude --version 2>$null | Select-Object -First 1) }
# Docker daemon and compose v2
$daemon = 'MISSING'
try { docker info --format '{{.ServerVersion}}' 2>$null | Out-Null; if ($LASTEXITCODE -eq 0) { $daemon = 'ok' } } catch {}
$rows += [pscustomobject]@{ Tool='docker daemon'; Status=$daemon; Version=''; Need='running'; Get='Start Docker Desktop'; Required=$true }
$compose = 'MISSING'
try { $cv = docker compose version --short 2>$null; if ($LASTEXITCODE -eq 0 -and $cv) { $compose = 'ok' } } catch {}
$rows += [pscustomobject]@{ Tool='docker compose'; Status=$compose; Version=$cv; Need='v2'; Get='Included with Docker Desktop'; Required=$true }
$rows | Format-Table Tool, Status, Version, Need, Get -AutoSize
$missing = $rows | Where-Object { $_.Required -and $_.Status -ne 'ok' }
if ($missing) { Write-Host "`nRequired tools missing or too old: $($missing.Tool -join ', ')" -ForegroundColor Red; exit 1 }
Write-Host "`nEverything required is installed." -ForegroundColor Green
