param(
  [string]$RagBaseUrl = "http://localhost:8001",
  [string]$KnowledgeDir = (Join-Path $PSScriptRoot "..\knowledge\scenarios")
)

$resolved = Resolve-Path -LiteralPath $KnowledgeDir
$base = $RagBaseUrl.TrimEnd("/")
$files = Get-ChildItem -LiteralPath $resolved.Path -Filter "*.md"

if ($files.Count -eq 0) {
  throw "No markdown files found in $($resolved.Path)"
}

foreach ($file in $files) {
  $category = [System.IO.Path]::GetFileNameWithoutExtension($file.Name)
  $filePath = [System.Uri]::EscapeDataString($file.FullName)
  $categoryParam = [System.Uri]::EscapeDataString($category)
  $uri = "$base/rag/hybrid/write?filePath=$filePath&category=$categoryParam"
  Write-Host "Ingesting $($file.Name) as category '$category'"
  Invoke-RestMethod -Method Post -Uri $uri | Out-Null
}

Write-Host "RAG knowledge ingestion completed."
