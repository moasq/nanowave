package icons

// Spec defines a required icon size entry.
type Spec struct {
	Idiom    string
	Size     string // e.g. "60x60"
	Scale    string // "1x", "2x", "3x"
	Pixels   int    // actual pixel dimension (size * scale)
	Filename string // generated filename
}

// IOSSpecs returns all icon sizes required for iOS App Store submission.
func IOSSpecs() []Spec {
	return []Spec{
		// iPhone Notification
		{Idiom: "iphone", Size: "20x20", Scale: "2x", Pixels: 40, Filename: "Icon-20@2x.png"},
		{Idiom: "iphone", Size: "20x20", Scale: "3x", Pixels: 60, Filename: "Icon-20@3x.png"},
		// iPhone Settings
		{Idiom: "iphone", Size: "29x29", Scale: "2x", Pixels: 58, Filename: "Icon-29@2x.png"},
		{Idiom: "iphone", Size: "29x29", Scale: "3x", Pixels: 87, Filename: "Icon-29@3x.png"},
		// iPhone Spotlight
		{Idiom: "iphone", Size: "40x40", Scale: "2x", Pixels: 80, Filename: "Icon-40@2x.png"},
		{Idiom: "iphone", Size: "40x40", Scale: "3x", Pixels: 120, Filename: "Icon-40@3x.png"},
		// iPhone App
		{Idiom: "iphone", Size: "60x60", Scale: "2x", Pixels: 120, Filename: "Icon-60@2x.png"},
		{Idiom: "iphone", Size: "60x60", Scale: "3x", Pixels: 180, Filename: "Icon-60@3x.png"},
		// iPad Notification
		{Idiom: "ipad", Size: "20x20", Scale: "1x", Pixels: 20, Filename: "Icon-20.png"},
		{Idiom: "ipad", Size: "20x20", Scale: "2x", Pixels: 40, Filename: "Icon-20-ipad@2x.png"},
		// iPad Settings
		{Idiom: "ipad", Size: "29x29", Scale: "1x", Pixels: 29, Filename: "Icon-29.png"},
		{Idiom: "ipad", Size: "29x29", Scale: "2x", Pixels: 58, Filename: "Icon-29-ipad@2x.png"},
		// iPad Spotlight
		{Idiom: "ipad", Size: "40x40", Scale: "1x", Pixels: 40, Filename: "Icon-40.png"},
		{Idiom: "ipad", Size: "40x40", Scale: "2x", Pixels: 80, Filename: "Icon-40-ipad@2x.png"},
		// iPad App
		{Idiom: "ipad", Size: "76x76", Scale: "1x", Pixels: 76, Filename: "Icon-76.png"},
		{Idiom: "ipad", Size: "76x76", Scale: "2x", Pixels: 152, Filename: "Icon-76@2x.png"},
		// iPad Pro
		{Idiom: "ipad", Size: "83.5x83.5", Scale: "2x", Pixels: 167, Filename: "Icon-83.5@2x.png"},
		// App Store
		{Idiom: "ios-marketing", Size: "1024x1024", Scale: "1x", Pixels: 1024, Filename: "Icon-1024.png"},
	}
}
