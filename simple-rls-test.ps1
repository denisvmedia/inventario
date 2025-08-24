# Simple RLS Testing Script for Inventario
# Tests Row Level Security implementation

param(
    [string]$BaseUrl = "http://localhost:3333/api/v1"
)

# Test users
$User1 = @{
    Email = "admin@test-org.com"
    Password = "testpassword123"
}

$User2 = @{
    Email = "user2@test-org.com"
    Password = "testpassword123"
}

# Function to login and get token with retry
function Get-AuthToken {
    param($User, [int]$MaxRetries = 3)

    $loginBody = @{
        email = $User.Email
        password = $User.Password
    } | ConvertTo-Json

    for ($i = 1; $i -le $MaxRetries; $i++) {
        try {
            $response = Invoke-WebRequest -Uri "$BaseUrl/auth/login" -Method POST -ContentType "application/json" -Body $loginBody
            $loginData = $response.Content | ConvertFrom-Json
            Write-Host "SUCCESS: $($User.Email) authenticated (attempt $i)" -ForegroundColor Green
            return $loginData.token
        }
        catch {
            Write-Host "ATTEMPT $i FAILED: $($User.Email) login failed: $_" -ForegroundColor Yellow
            if ($i -lt $MaxRetries) {
                Start-Sleep -Seconds 1
            }
        }
    }

    Write-Host "FAILED: $($User.Email) authentication failed after $MaxRetries attempts" -ForegroundColor Red
    return $null
}

# Function to make authenticated API call
function Invoke-AuthenticatedRequest {
    param(
        [string]$Uri,
        [string]$Method = "GET",
        [string]$Token,
        [string]$Body = $null,
        [string]$ContentType = "application/json"
    )

    $headers = @{
        "Authorization" = "Bearer $Token"
    }

    try {
        if ($Body) {
            $response = Invoke-WebRequest -Uri $Uri -Method $Method -Headers $headers -ContentType $ContentType -Body $Body
        } else {
            $response = Invoke-WebRequest -Uri $Uri -Method $Method -Headers $headers
        }
        return @{
            Success = $true
            StatusCode = $response.StatusCode
            Content = $response.Content | ConvertFrom-Json
        }
    }
    catch {
        return @{
            Success = $false
            StatusCode = if ($_.Exception.Response) { $_.Exception.Response.StatusCode.value__ } else { 0 }
            Error = $_.Exception.Message
        }
    }
}

# Function to create test data for a user
function Create-TestData {
    param(
        [string]$UserToken,
        [string]$UserName
    )

    Write-Host "Creating test data for $UserName..." -ForegroundColor Cyan

    # Create a location
    $locationData = @{
        data = @{
            type = "locations"
            attributes = @{
                name = "Test Location by $UserName"
                address = "Test Address for $UserName"
            }
        }
    } | ConvertTo-Json -Depth 3

    $locationResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Method POST -Token $UserToken -Body $locationData

    if ($locationResult.Success) {
        $locationId = $locationResult.Content.data.id
        Write-Host "  Created location: $($locationResult.Content.data.attributes.name)" -ForegroundColor Green

        # Create an area in the location
        $areaData = @{
            data = @{
                type = "areas"
                attributes = @{
                    name = "Test Area by $UserName"
                    location_id = $locationId
                }
            }
        } | ConvertTo-Json -Depth 3

        $areaResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/areas" -Method POST -Token $UserToken -Body $areaData

        if ($areaResult.Success) {
            Write-Host "  Created area: $($areaResult.Content.data.attributes.name)" -ForegroundColor Green
            return @{
                LocationId = $locationId
                AreaId = $areaResult.Content.data.id
                LocationName = $locationResult.Content.data.attributes.name
                AreaName = $areaResult.Content.data.attributes.name
            }
        } else {
            Write-Host "  FAILED to create area: $($areaResult.Error)" -ForegroundColor Red
        }
    } else {
        Write-Host "  FAILED to create location: $($locationResult.Error)" -ForegroundColor Red
    }

    return $null
}

# Function to test bi-directional isolation
function Test-BiDirectionalIsolation {
    param(
        [string]$EntityType,
        [string]$ListEndpoint,
        [string]$User1Token,
        [string]$User2Token,
        [hashtable]$User1Data,
        [hashtable]$User2Data
    )

    Write-Host "`nTesting bi-directional isolation for $EntityType" -ForegroundColor Yellow

    # Test User 1 can see their own data
    Write-Host "  User 1 accessing $EntityType..." -ForegroundColor Cyan
    $user1Result = Invoke-AuthenticatedRequest -Uri "$BaseUrl$ListEndpoint" -Token $User1Token

    $user1CanSeeOwn = $false
    $user1CanSeeOther = $false

    if ($user1Result.Success) {
        $user1Items = $user1Result.Content.data
        $user1Count = $user1Items.Count
        Write-Host "    User 1 can see $user1Count $EntityType(s)" -ForegroundColor Gray

        # Check if User 1 can see their own data
        if ($EntityType -eq "Locations" -and $User1Data) {
            $user1CanSeeOwn = $user1Items | Where-Object { $_.attributes.name -eq $User1Data.LocationName }
        } elseif ($EntityType -eq "Areas" -and $User1Data) {
            $user1CanSeeOwn = $user1Items | Where-Object { $_.attributes.name -eq $User1Data.AreaName }
        }

        # Check if User 1 can see User 2's data (should not)
        if ($EntityType -eq "Locations" -and $User2Data) {
            $user1CanSeeOther = $user1Items | Where-Object { $_.attributes.name -eq $User2Data.LocationName }
        } elseif ($EntityType -eq "Areas" -and $User2Data) {
            $user1CanSeeOther = $user1Items | Where-Object { $_.attributes.name -eq $User2Data.AreaName }
        }
    } else {
        Write-Host "    FAILED: User 1 cannot access $EntityType" -ForegroundColor Red
        return $false
    }

    # Test User 2 can see their own data
    Write-Host "  User 2 accessing $EntityType..." -ForegroundColor Cyan
    $user2Result = Invoke-AuthenticatedRequest -Uri "$BaseUrl$ListEndpoint" -Token $User2Token

    $user2CanSeeOwn = $false
    $user2CanSeeOther = $false

    if ($user2Result.Success) {
        $user2Items = $user2Result.Content.data
        $user2Count = $user2Items.Count
        Write-Host "    User 2 can see $user2Count $EntityType(s)" -ForegroundColor Gray

        # Check if User 2 can see their own data
        if ($EntityType -eq "Locations" -and $User2Data) {
            $user2CanSeeOwn = $user2Items | Where-Object { $_.attributes.name -eq $User2Data.LocationName }
        } elseif ($EntityType -eq "Areas" -and $User2Data) {
            $user2CanSeeOwn = $user2Items | Where-Object { $_.attributes.name -eq $User2Data.AreaName }
        }

        # Check if User 2 can see User 1's data (should not)
        if ($EntityType -eq "Locations" -and $User1Data) {
            $user2CanSeeOther = $user2Items | Where-Object { $_.attributes.name -eq $User1Data.LocationName }
        } elseif ($EntityType -eq "Areas" -and $User1Data) {
            $user2CanSeeOther = $user2Items | Where-Object { $_.attributes.name -eq $User1Data.AreaName }
        }
    } else {
        Write-Host "    User 2 access denied (could be RLS working)" -ForegroundColor Yellow
    }

    # Evaluate results
    $success = $true

    if ($user1CanSeeOwn) {
        Write-Host "    SUCCESS: User 1 can see their own $EntityType" -ForegroundColor Green
    } else {
        Write-Host "    FAILED: User 1 cannot see their own $EntityType" -ForegroundColor Red
        $success = $false
    }

    if ($user1CanSeeOther) {
        Write-Host "    FAILED: User 1 can see User 2's $EntityType (RLS violation)" -ForegroundColor Red
        $success = $false
    } else {
        Write-Host "    SUCCESS: User 1 cannot see User 2's $EntityType" -ForegroundColor Green
    }

    if ($user2CanSeeOwn) {
        Write-Host "    SUCCESS: User 2 can see their own $EntityType" -ForegroundColor Green
    } else {
        Write-Host "    FAILED: User 2 cannot see their own $EntityType" -ForegroundColor Red
        $success = $false
    }

    if ($user2CanSeeOther) {
        Write-Host "    FAILED: User 2 can see User 1's $EntityType (RLS violation)" -ForegroundColor Red
        $success = $false
    } else {
        Write-Host "    SUCCESS: User 2 cannot see User 1's $EntityType" -ForegroundColor Green
    }

    return $success
}

# Simple single-user test function
function Test-SingleUserRLS {
    param(
        [string]$UserToken,
        [string]$UserName
    )

    Write-Host "`n=== Testing Single User RLS ===" -ForegroundColor Magenta
    Write-Host "Testing user: $UserName" -ForegroundColor Cyan

    # Test user can access locations
    Write-Host "Testing access to locations..." -ForegroundColor Cyan
    $locationsResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Token $UserToken

    if ($locationsResult.Success) {
        $count = $locationsResult.Content.data.Count
        Write-Host "SUCCESS: User can see $count locations" -ForegroundColor Green
    } else {
        Write-Host "FAILED: User cannot access locations: $($locationsResult.Error)" -ForegroundColor Red
        return
    }

    # Test user can create a location
    Write-Host "Testing location creation..." -ForegroundColor Cyan
    $locationData = @{
        data = @{
            type = "locations"
            attributes = @{
                name = "Test Location by $UserName"
                address = "Test Address"
            }
        }
    } | ConvertTo-Json -Depth 3

    $createResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Method POST -Token $UserToken -Body $locationData

    if ($createResult.Success) {
        Write-Host "SUCCESS: User can create locations" -ForegroundColor Green
        Write-Host "Created location: $($createResult.Content.data.attributes.name)" -ForegroundColor Green
    } else {
        Write-Host "FAILED: User cannot create locations: $($createResult.Error)" -ForegroundColor Red
    }

    Write-Host "`nSingle user test completed" -ForegroundColor Yellow
}

# Simple function to test basic RLS functionality
function Test-RLS-Simple {
    Write-Host "Starting Simple RLS Testing..." -ForegroundColor Yellow
    Write-Host "Base URL: $BaseUrl" -ForegroundColor Gray

    # Step 1: Authentication
    Write-Host "`n=== STEP 1: Authentication ===" -ForegroundColor Magenta
    $user1Token = Get-AuthToken -User $User1 -MaxRetries 3
    $user2Token = Get-AuthToken -User $User2 -MaxRetries 3

    if (-not $user1Token) {
        Write-Host "FAILED: User 1 authentication failed" -ForegroundColor Red
        return
    }

    if (-not $user2Token) {
        Write-Host "WARNING: User 2 authentication failed, testing with User 1 only" -ForegroundColor Yellow
        Test-SingleUserRLS -UserToken $user1Token -UserName $User1.Email
        return
    }

    Write-Host "SUCCESS: Both users authenticated" -ForegroundColor Green

    # Step 2: Test data creation for both users
    Write-Host "`n=== STEP 2: Test Data Creation ===" -ForegroundColor Magenta

    # User 1 creates a location
    Write-Host "User 1 creating location..." -ForegroundColor Cyan
    $user1LocationData = @{
        data = @{
            type = "locations"
            attributes = @{
                name = "User 1 Test Location"
                address = "User 1 Address"
            }
        }
    } | ConvertTo-Json -Depth 3

    $user1CreateResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Method POST -Token $user1Token -Body $user1LocationData

    if ($user1CreateResult.Success) {
        $user1LocationId = $user1CreateResult.Content.data.id
        Write-Host "  SUCCESS: User 1 created location with ID: $user1LocationId" -ForegroundColor Green
    } else {
        Write-Host "  FAILED: User 1 cannot create location: $($user1CreateResult.Error)" -ForegroundColor Red
        return
    }

    # User 2 creates a location
    Write-Host "User 2 creating location..." -ForegroundColor Cyan
    $user2LocationData = @{
        data = @{
            type = "locations"
            attributes = @{
                name = "User 2 Test Location"
                address = "User 2 Address"
            }
        }
    } | ConvertTo-Json -Depth 3

    $user2CreateResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Method POST -Token $user2Token -Body $user2LocationData

    if ($user2CreateResult.Success) {
        $user2LocationId = $user2CreateResult.Content.data.id
        Write-Host "  SUCCESS: User 2 created location with ID: $user2LocationId" -ForegroundColor Green
    } else {
        Write-Host "  FAILED: User 2 cannot create location: $($user2CreateResult.Error)" -ForegroundColor Red
        return
    }

    # Step 3: Test RLS isolation
    Write-Host "`n=== STEP 3: Test RLS Isolation ===" -ForegroundColor Magenta

    # Test User 1 can access their own location by ID
    Write-Host "Testing User 1 accessing their own location by ID..." -ForegroundColor Cyan
    $user1OwnAccessResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations/$user1LocationId" -Token $user1Token

    if ($user1OwnAccessResult.Success) {
        Write-Host "  SUCCESS: User 1 can access their own location" -ForegroundColor Green
    } else {
        Write-Host "  FAILED: User 1 cannot access their own location: $($user1OwnAccessResult.StatusCode)" -ForegroundColor Red
    }

    # Test User 1 CANNOT access User 2's location by ID
    Write-Host "Testing User 1 accessing User 2's location by ID..." -ForegroundColor Cyan
    $user1CrossAccessResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations/$user2LocationId" -Token $user1Token

    if ($user1CrossAccessResult.Success) {
        Write-Host "  FAILED: User 1 can access User 2's location (RLS violation!)" -ForegroundColor Red
    } else {
        Write-Host "  SUCCESS: User 1 cannot access User 2's location (RLS working) - Status: $($user1CrossAccessResult.StatusCode)" -ForegroundColor Green
    }

    # Test User 2 can access their own location by ID
    Write-Host "Testing User 2 accessing their own location by ID..." -ForegroundColor Cyan
    $user2OwnAccessResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations/$user2LocationId" -Token $user2Token

    if ($user2OwnAccessResult.Success) {
        Write-Host "  SUCCESS: User 2 can access their own location" -ForegroundColor Green
    } else {
        Write-Host "  FAILED: User 2 cannot access their own location: $($user2OwnAccessResult.StatusCode)" -ForegroundColor Red
    }

    # Test User 2 CANNOT access User 1's location by ID
    Write-Host "Testing User 2 accessing User 1's location by ID..." -ForegroundColor Cyan
    $user2CrossAccessResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations/$user1LocationId" -Token $user2Token

    if ($user2CrossAccessResult.Success) {
        Write-Host "  FAILED: User 2 can access User 1's location (RLS violation!)" -ForegroundColor Red
    } else {
        Write-Host "  SUCCESS: User 2 cannot access User 1's location (RLS working) - Status: $($user2CrossAccessResult.StatusCode)" -ForegroundColor Green
    }

    # Step 4: Test list isolation
    Write-Host "`n=== STEP 4: Test List Isolation ===" -ForegroundColor Magenta

    # Test User 1 list view
    Write-Host "Testing User 1 list view..." -ForegroundColor Cyan
    $user1ListResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Token $user1Token

    if ($user1ListResult.Success) {
        $user1Count = $user1ListResult.Content.data.Count
        Write-Host "  User 1 can see $user1Count locations" -ForegroundColor Green
    } else {
        Write-Host "  FAILED: User 1 cannot list locations" -ForegroundColor Red
    }

    # Test User 2 list view
    Write-Host "Testing User 2 list view..." -ForegroundColor Cyan
    $user2ListResult = Invoke-AuthenticatedRequest -Uri "$BaseUrl/locations" -Token $user2Token

    if ($user2ListResult.Success) {
        $user2Count = $user2ListResult.Content.data.Count
        Write-Host "  User 2 can see $user2Count locations" -ForegroundColor Green

        # Compare results
        if ($user1Count -ne $user2Count) {
            Write-Host "  SUCCESS: Users see different data in lists (RLS working)" -ForegroundColor Green
        } else {
            Write-Host "  WARNING: Users see same amount of data (check RLS)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  FAILED: User 2 cannot list locations" -ForegroundColor Red
    }

    Write-Host "`n=== RLS Test Summary ===" -ForegroundColor Magenta
    Write-Host "- Both users should be able to create their own data" -ForegroundColor Gray
    Write-Host "- Users should be able to access their own data by ID" -ForegroundColor Gray
    Write-Host "- Users should NOT be able to access other users' data by ID" -ForegroundColor Gray
    Write-Host "- Users should see only their own data in lists" -ForegroundColor Gray
}

# Run the tests
Test-RLS-Simple
