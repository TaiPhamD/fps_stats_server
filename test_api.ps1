# FPS Server API Test Script
# This script tests all the API endpoints to verify they're working

$baseUrl = "http://localhost:8080"

Write-Host "Testing FPS Server API..." -ForegroundColor Cyan
Write-Host "Base URL: $baseUrl" -ForegroundColor Gray
Write-Host ""

# Test root endpoint
Write-Host "Testing root endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "Service: $($data.service)" -ForegroundColor Green
    Write-Host "Version: $($data.version)" -ForegroundColor Green
    Write-Host "Available endpoints: $($data.endpoints.Keys -join ', ')" -ForegroundColor Green
} catch {
    Write-Host "Root endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test FPS endpoint
Write-Host "Testing FPS endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/fps" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "FPS sensors: $($data.Count)" -ForegroundColor Green
    
    # Show FPS sensors
    $data | ForEach-Object {
        $value = if ($_.value -eq $null) { "null" } else { $_.value }
        Write-Host "  $($_.name): $value $($_.unit)" -ForegroundColor Gray
    }
} catch {
    Write-Host "FPS endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test GPU endpoint
Write-Host "Testing GPU endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/gpu" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "GPU sensors: $($data.Count)" -ForegroundColor Green
    
    # Show first few GPU sensors
    $data | Select-Object -First 3 | ForEach-Object {
        $value = if ($_.value -eq $null) { "null" } else { $_.value }
        Write-Host "  $($_.name): $value $($_.unit)" -ForegroundColor Gray
    }
} catch {
    Write-Host "GPU endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test CPU endpoint
Write-Host "Testing CPU endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/cpu" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "CPU sensors: $($data.Count)" -ForegroundColor Green
    
    # Show first few CPU sensors
    $data | Select-Object -First 3 | ForEach-Object {
        $value = if ($_.value -eq $null) { "null" } else { $_.value }
        Write-Host "  $($_.name): $value $($_.unit)" -ForegroundColor Gray
    }
} catch {
    Write-Host "CPU endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test Memory endpoint
Write-Host "Testing Memory endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/memory" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "Memory sensors: $($data.Count)" -ForegroundColor Green
    
    $data | ForEach-Object {
        $value = if ($_.value -eq $null) { "null" } else { $_.value }
        Write-Host "  $($_.name): $value $($_.unit)" -ForegroundColor Gray
    }
} catch {
    Write-Host "Memory endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test Memory Stats endpoint
Write-Host "Testing Memory Stats endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/debug/memory" -UseBasicParsing
    $data = $response.Content | ConvertFrom-Json
    Write-Host "Heap Alloc: $($data.heap_alloc_mb) MB" -ForegroundColor Green
    Write-Host "Total Alloc: $($data.total_alloc_mb) MB" -ForegroundColor Green
    Write-Host "Heap InUse: $($data.heap_inuse_mb) MB" -ForegroundColor Green
    Write-Host "Goroutines: $($data.num_goroutines)" -ForegroundColor Green
    Write-Host "GC Count: $($data.num_gc)" -ForegroundColor Green
    Write-Host "Timestamp: $($data.timestamp)" -ForegroundColor Green
} catch {
    Write-Host "Memory Stats endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "API testing completed!" -ForegroundColor Green
Write-Host ""
Write-Host "Available endpoints:" -ForegroundColor Cyan
Write-Host "  Root: GET $baseUrl/" -ForegroundColor White
Write-Host "  FPS: GET $baseUrl/fps" -ForegroundColor White
Write-Host "  GPU: GET $baseUrl/gpu" -ForegroundColor White
Write-Host "  CPU: GET $baseUrl/cpu" -ForegroundColor White
Write-Host "  Memory: GET $baseUrl/memory" -ForegroundColor White
Write-Host "  Debug Memory: GET $baseUrl/debug/memory" -ForegroundColor White
Write-Host ""
Write-Host "For HomeAssistant integration, use these endpoints:" -ForegroundColor Cyan
Write-Host "  FPS: GET $baseUrl/fps" -ForegroundColor White
Write-Host "  GPU: GET $baseUrl/gpu" -ForegroundColor White
Write-Host "  CPU: GET $baseUrl/cpu" -ForegroundColor White
Write-Host "  Memory: GET $baseUrl/memory" -ForegroundColor White
Write-Host ""
Write-Host "To build the tray application:" -ForegroundColor Cyan
Write-Host "  go build -ldflags=\"-H windowsgui\" -o fps_tray.exe fps_tray.go" -ForegroundColor White
Write-Host "" 