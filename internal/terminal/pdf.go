package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertPDFToImages renders each page of a PDF as a PNG image using macOS
// CoreGraphics. Returns the list of generated PNG paths. Falls back to sips
// for single-page PDFs if the Swift script is unavailable.
func ConvertPDFToImages(pdfPath, outputDir string) ([]string, error) {
	baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))

	// Build the output path prefix that the Swift script will use.
	// Each page becomes: <outputDir>/<baseName>_page_<N>.png
	outPrefix := filepath.Join(outputDir, baseName)

	// Use a small Swift script via CoreGraphics — handles multi-page PDFs.
	// We pass pdfPath and outPrefix as command-line arguments to avoid
	// fmt.Sprintf conflicts with Swift's String(format:) specifiers.
	script := `
import Foundation
import CoreGraphics
import ImageIO

let pdfPath = CommandLine.arguments[1]
let outPrefix = CommandLine.arguments[2]

guard let url = CFURLCreateWithFileSystemPath(nil, pdfPath as CFString, .cfurlposixPathStyle, false),
      let doc = CGPDFDocument(url) else {
    fputs("error: cannot open PDF\n", stderr)
    exit(1)
}
let pageCount = doc.numberOfPages
for i in 1...pageCount {
    guard let page = doc.page(at: i) else { continue }
    let rect = page.getBoxRect(.mediaBox)
    let scale: CGFloat = 2.0
    let w = Int(rect.width * scale)
    let h = Int(rect.height * scale)
    let cs = CGColorSpaceCreateDeviceRGB()
    guard let ctx = CGContext(data: nil, width: w, height: h,
                             bitsPerComponent: 8, bytesPerRow: 0, space: cs,
                             bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue) else { continue }
    ctx.setFillColor(CGColor(red: 1, green: 1, blue: 1, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: w, height: h))
    ctx.scaleBy(x: scale, y: scale)
    ctx.drawPDFPage(page)
    guard let image = ctx.makeImage() else { continue }
    let outPath = "\(outPrefix)_page_\(i).png"
    guard let dest = CGImageDestinationCreateWithURL(
        CFURLCreateWithFileSystemPath(nil, outPath as CFString, .cfurlposixPathStyle, false),
        "public.png" as CFString, 1, nil) else { continue }
    CGImageDestinationAddImage(dest, image, nil)
    CGImageDestinationFinalize(dest)
    print(outPath)
}
`

	cmd := exec.Command("swift", "-", pdfPath, outPrefix)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err == nil {
		var pages []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				if info, statErr := os.Stat(line); statErr == nil && info.Size() > 0 {
					pages = append(pages, line)
				}
			}
		}
		if len(pages) > 0 {
			return pages, nil
		}
	}

	// Fallback: sips converts the first page only.
	outPath := outPrefix + "_page_1.png"
	sipsCmd := exec.Command("sips", "-s", "format", "png", pdfPath, "--out", outPath)
	if err := sipsCmd.Run(); err != nil {
		return nil, fmt.Errorf("pdf conversion failed: %w", err)
	}
	if info, err := os.Stat(outPath); err == nil && info.Size() > 0 {
		return []string{outPath}, nil
	}
	return nil, fmt.Errorf("pdf conversion produced no output")
}

// IsPDF returns true if the path has a .pdf extension.
func IsPDF(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".pdf"
}
