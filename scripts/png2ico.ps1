
param(
    [string]$SourcePng,
    [string]$DestIco
)

Add-Type -AssemblyName System.Drawing

$srcImageRaw = [System.Drawing.Bitmap]::FromFile($SourcePng)
$srcImage = New-Object System.Drawing.Bitmap(256, 256)
$g = [System.Drawing.Graphics]::FromImage($srcImage)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.DrawImage($srcImageRaw, 0, 0, 256, 256)
$g.Dispose()
$srcImageRaw.Dispose()

$iconStream = [System.IO.File]::Create($DestIco)

# ICO Header
# Reserved (2), Type (2) (1=Icon), Count (2)
$header = [byte[]]@(0, 0, 1, 0, 1, 0)
$iconStream.Write($header, 0, 6)

# Image Directory Entry
$width = 0   # 0 means 256
$height = 0  # 0 means 256

# Get PNG data
$ms = New-Object System.IO.MemoryStream
$srcImage.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
$pngData = $ms.ToArray()
$size = $pngData.Length

# Offset: 6 (Header) + 16 (Dir Entry) = 22
$offset = 22

# Width (1), Height (1), Colors (1), Reserved (1), Planes (2), BPP (2), Size (4), Offset (4)
# Colors=0 (>=8bpp), Planes=1, BPP=32
$entry = [byte[]]@($width, $height, 0, 0, 1, 0, 32, 0)
$sizeBytes = [BitConverter]::GetBytes([int]$size)
$offsetBytes = [BitConverter]::GetBytes([int]$offset)

$iconStream.Write($entry, 0, 8)
$iconStream.Write($sizeBytes, 0, 4)
$iconStream.Write($offsetBytes, 0, 4)

# Image Data
$iconStream.Write($pngData, 0, $pngData.Length)

$iconStream.Close()
$srcImage.Dispose()
$ms.Dispose()

Write-Host "Created ICO: $DestIco (256x256)"
