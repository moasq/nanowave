---
name: "game-assets"
description: "Download free game sprites/textures/3D models and generate procedural assets. Covers nw_download_asset tool, texture factories, sprite atlas organization, 3D model loading, and programmatic asset creation."
---
# Game Asset Management

## Downloading Assets

Use `nw_download_asset` to download sprites, backgrounds, textures, sounds, and 3D models from free CC0 sources:

```
nw_download_asset:
  project_dir: /path/to/project
  app_name: MyGame
  url: https://example.com/sprite.png   (HTTPS only)
  asset_name: paddle                     (becomes Assets.xcassets/paddle.imageset/)
  asset_kind: sprite | background | texture | sound | model
```

After download, use in SpriteKit:
```swift
let texture = SKTexture(imageNamed: "paddle")
let sprite = SKSpriteNode(texture: texture)
```

After download, use in SceneKit (3D models):
```swift
let scene = SCNScene(named: "car.usdz")!
let modelNode = scene.rootNode.childNodes.first!
```

Trusted free sources:
- **2D**: Kenney.nl (CC0), OpenGameArt.org (mixed CC0/CC-BY), GitHub game asset repos
- **3D**: Sketchfab (CC0/CC-BY USDZ/OBJ), Poly Pizza (CC0), Kenney.nl 3D assets (CC0 OBJ/DAE)

## Procedural Asset Generation

Generate sprites in code — no external files needed:

```swift
import UIKit
import SpriteKit

class TextureFactory {
    static func circle(diameter: CGFloat, color: UIColor) -> SKTexture {
        let renderer = UIGraphicsImageRenderer(size: CGSize(width: diameter, height: diameter))
        let image = renderer.image { ctx in
            color.setFill()
            ctx.cgContext.fillEllipse(in: CGRect(x: 0, y: 0, width: diameter, height: diameter))
        }
        return SKTexture(image: image)
    }

    static func roundedRect(size: CGSize, color: UIColor, cornerRadius: CGFloat = 8) -> SKTexture {
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            color.setFill()
            UIBezierPath(roundedRect: CGRect(origin: .zero, size: size), cornerRadius: cornerRadius).fill()
        }
        return SKTexture(image: image)
    }

    static func gradient(size: CGSize, colors: [UIColor]) -> SKTexture {
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            let cgColors = colors.map { $0.cgColor } as CFArray
            let gradient = CGGradient(colorsSpace: CGColorSpaceCreateDeviceRGB(), colors: cgColors, locations: nil)!
            ctx.cgContext.drawLinearGradient(gradient,
                start: .zero, end: CGPoint(x: 0, y: size.height),
                options: [])
        }
        return SKTexture(image: image)
    }
}
```

**Always use SKSpriteNode with generated textures** — SKShapeNode is significantly slower and drops FPS. Convert shapes:
```swift
let shape = SKShapeNode(circleOfRadius: 15)
shape.fillColor = .white
let texture = view.texture(from: shape)!
let sprite = SKSpriteNode(texture: texture) // Use this instead of shape
```

## Asset Catalog Organization

```
Assets.xcassets/
├── AppIcon.appiconset/
├── AccentColor.colorset/
├── Game/
│   ├── player.imageset/       (individual game sprites)
│   ├── enemy.imageset/
│   ├── background.imageset/
│   └── UI_Atlas.spriteatlas/  (grouped UI sprites for performance)
```

Rules:
- Group related sprites into `.spriteatlas` folders for rendering performance
- Split atlases by scene/level — do NOT put everything in one atlas
- Keep texture dimensions in powers of 2 when possible (256, 512, 1024)
- Cache generated textures — do not regenerate each frame

## Sound Effects — Procedural Generation

Generate WAV data in memory — no audio files needed:

```swift
func generateTone(frequency: Float, duration: Float, volume: Float = 0.5) -> AVAudioPlayer? {
    let sampleRate: Float = 44100
    let samples = Int(sampleRate * duration)
    var data = Data()
    // ... WAV header + PCM samples (see spritekit-game skill for full implementation)
    return try? AVAudioPlayer(data: data)
}
```

Common game sound recipes:
- **Hit/bounce**: 440-880 Hz, 0.05-0.1s
- **Score point**: Two tones (660 Hz → 880 Hz), 0.1s each
- **Game over**: 220 Hz descending envelope, 0.3s
- **Power-up**: Rising sweep 330→880 Hz, 0.2s

Alternative: `SKAction.playSoundFileNamed("hit.wav", waitForCompletion: false)` if you download WAV files via `nw_download_asset`.

## Fallback: Programmatic Sprites

If no suitable download exists, generate colored shapes:

```swift
// Paddle
let paddle = SKSpriteNode(texture: TextureFactory.roundedRect(
    size: CGSize(width: 120, height: 20),
    color: .systemBlue,
    cornerRadius: 10
))

// Ball
let ball = SKSpriteNode(texture: TextureFactory.circle(
    diameter: 20,
    color: .white
))

// Background
let bg = SKSpriteNode(texture: TextureFactory.gradient(
    size: scene.size,
    colors: [UIColor(red: 0, green: 0.3, blue: 0, alpha: 1), UIColor(red: 0, green: 0.2, blue: 0, alpha: 1)]
))
```

Downloaded or well-crafted procedural assets always look better than plain colored rectangles.

## 3D Models for SceneKit

Use `nw_download_asset` with `asset_kind: "model"` to download 3D models (USDZ, OBJ, DAE, SCN):

```
nw_download_asset:
  project_dir: /path/to/project
  app_name: MyGame
  url: https://example.com/car.usdz
  asset_name: car
  asset_kind: model
```

Models are placed in `<AppName>/Models/` and loaded via:
```swift
// USDZ/OBJ via ModelIO
import ModelIO
import SceneKit.ModelIO
let url = Bundle.main.url(forResource: "car", withExtension: "usdz")!
let modelNode = SCNReferenceNode(url: url)

// SCN/DAE directly
let scene = SCNScene(named: "car.scn")!
let modelNode = scene.rootNode.childNodes.first!
```

### Procedural 3D — No Models Needed

Many 3D games can be built entirely from SceneKit primitives:
```swift
// Bowling pin from cylinder + sphere
let pinBody = SCNCylinder(radius: 0.15, height: 0.8)
let pinHead = SCNSphere(radius: 0.18)

// Race car from boxes
let carBody = SCNBox(width: 1.5, height: 0.4, length: 3.0, chamferRadius: 0.1)
let wheel = SCNCylinder(radius: 0.3, height: 0.15)

// Board game piece from capsule
let piece = SCNCapsule(capRadius: 0.2, height: 0.6)
```

Use programmatic materials (diffuse color, metalness, roughness) instead of texture files for clean, stylized looks.

### 3D Material Textures

Generate textures for 3D materials programmatically:
```swift
class TextureFactory3D {
    static func solidColor(_ color: UIColor, size: CGFloat = 64) -> UIImage {
        let renderer = UIGraphicsImageRenderer(size: CGSize(width: size, height: size))
        return renderer.image { ctx in
            color.setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: size, height: size))
        }
    }

    static func woodGrain(size: CGFloat = 256) -> UIImage {
        let renderer = UIGraphicsImageRenderer(size: CGSize(width: size, height: size))
        return renderer.image { ctx in
            UIColor.brown.setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: size, height: size))
            UIColor(white: 0.3, alpha: 0.3).setFill()
            for i in stride(from: 0, to: size, by: 8) {
                ctx.fill(CGRect(x: 0, y: i, width: size, height: 2))
            }
        }
    }
}

// Apply to material
material.diffuse.contents = TextureFactory3D.woodGrain()
```
