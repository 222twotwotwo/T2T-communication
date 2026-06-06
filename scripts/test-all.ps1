Push-Location (Join-Path $PSScriptRoot "..")
try {
  go test ./backend/... ./rag-go/...
} finally {
  Pop-Location
}
