---
name: "spritekit-game"
description: "Use for 2D games: arcade, puzzle, sports, ping pong, platformer, shooter, 2D racing. SpriteKit + SpriteView architecture, scene hierarchy, physics, game loop, audio, particles, game feel."
---
# SpriteKit 2D Game Development

**This skill is for 2D games only.** For 3D games (racing, bowling, board games, marble maze, tower defense, 3D sports), load the `scenekit-game` skill instead.

**ARCHITECTURE OVERRIDE**: 2D games use SpriteKit (SKScene + SpriteView), NOT plain SwiftUI views with timers or Canvas. The standard @main App → RootView → MainView pattern still applies, but MainView hosts a SpriteView instead of standard SwiftUI content. AppTheme rules apply to SwiftUI parts (menus, overlays) but NOT to SpriteKit scene internals where you use SKColor, SKLabelNode fonts, etc. directly.

Also load the `game-assets` skill for downloading sprites and textures via `nw_download_asset`.

## SpriteView — SwiftUI Bridge

```swift
import SpriteKit
import SwiftUI

struct GameView: View {
    @State var gameState = GameState()

    var body: some View {
        ZStack {
            SpriteView(scene: makeScene(), isPaused: gameState.isPaused)
                .ignoresSafeArea()

            // SwiftUI overlays for HUD, menus (use AppTheme here)
            if gameState.isPaused {
                PauseMenuView(gameState: gameState)
            }
        }
    }

    private func makeScene() -> GameScene {
        let scene = GameScene(size: UIScreen.main.bounds.size)
        scene.scaleMode = .resizeFill
        scene.gameState = gameState
        return scene
    }
}
```

Rules:
- Use `.scaleMode = .resizeFill` so the scene adapts to actual view size
- Pass data between SwiftUI and SpriteKit via `@Observable` game state objects
- SwiftUI overlays (menus, HUD, pause) sit in a ZStack above SpriteView
- Use `SpriteView(scene:, isPaused:)` to control pause from SwiftUI
- SwiftUI parts use AppTheme. SpriteKit scene internals use SKColor/SKLabelNode directly.

## Scene Architecture

Layer-based node hierarchy — create in `didMove(to:)`:

```
SKScene (GameScene)
├── backgroundLayer (SKNode) — z: -100
├── gameplayLayer (SKNode)  — z: 0
└── hudLayer (SKNode)       — z: 100
```

Rules:
- Set `zPosition` on layers, not individual sprites
- Camera: attach `SKCameraNode` to gameplayLayer, set `scene.camera`
- Scene transitions: `view?.presentScene(newScene, transition: .push(with: .up, duration: 0.3))`
- Clean up in `willMove(from:)` — remove actions, nil out references

## Physics

```swift
struct PhysicsCategory {
    static let none:    UInt32 = 0
    static let player:  UInt32 = 0x1 << 0
    static let enemy:   UInt32 = 0x1 << 1
    static let ball:    UInt32 = 0x1 << 2
    static let wall:    UInt32 = 0x1 << 3
}
```

Rules:
- Set `categoryBitMask`, `contactTestBitMask`, `collisionBitMask` on every physics body
- Implement `SKPhysicsContactDelegate` — use Double Dispatch (delegate to nodes, not massive if-else)
- Use rectangle/circle bodies for performance — avoid texture-based bodies
- Enable `usesPreciseCollisionDetection = true` only for fast objects (balls, bullets)
- Set `physicsWorld.gravity = .zero` for top-down games, `CGVector(dx: 0, dy: -9.8)` for platformers

## Game Loop

```swift
private var lastUpdateTime: TimeInterval = 0

override func update(_ currentTime: TimeInterval) {
    let dt = lastUpdateTime == 0 ? 0 : currentTime - lastUpdateTime
    lastUpdateTime = currentTime

    updateGameLogic(deltaTime: dt)
}
```

Rules:
- Always calculate delta time — never assume fixed frame rate
- `update()` → game logic, `didEvaluateActions()` → post-action, `didSimulatePhysics()` → position corrections
- Use `@MainActor @objc` for CADisplayLink targets in Swift 6

## Game State with @Observable

```swift
@Observable
class GameState {
    var score: Int = 0
    var lives: Int = 3
    var isPaused: Bool = false
    var phase: GamePhase = .menu

    enum GamePhase {
        case menu, playing, paused, gameOver
    }
}
```

Share between SwiftUI and SKScene — SKScene updates properties, SwiftUI reacts automatically.

## Audio — Procedural Sound Effects

Generate sounds programmatically — no audio files needed:

```swift
import AVFoundation

func generateTone(frequency: Float, duration: Float) -> AVAudioPlayer? {
    let sampleRate: Float = 44100
    let samples = Int(sampleRate * duration)
    var data = Data()

    // WAV header
    let dataSize = samples * 2
    let fileSize = 36 + dataSize
    data.append(contentsOf: "RIFF".utf8)
    data.append(contentsOf: withUnsafeBytes(of: Int32(fileSize).littleEndian) { Array($0) })
    data.append(contentsOf: "WAVEfmt ".utf8)
    data.append(contentsOf: withUnsafeBytes(of: Int32(16).littleEndian) { Array($0) })
    data.append(contentsOf: withUnsafeBytes(of: Int16(1).littleEndian) { Array($0) })  // PCM
    data.append(contentsOf: withUnsafeBytes(of: Int16(1).littleEndian) { Array($0) })  // mono
    data.append(contentsOf: withUnsafeBytes(of: Int32(44100).littleEndian) { Array($0) })
    data.append(contentsOf: withUnsafeBytes(of: Int32(88200).littleEndian) { Array($0) })
    data.append(contentsOf: withUnsafeBytes(of: Int16(2).littleEndian) { Array($0) })
    data.append(contentsOf: withUnsafeBytes(of: Int16(16).littleEndian) { Array($0) })
    data.append(contentsOf: "data".utf8)
    data.append(contentsOf: withUnsafeBytes(of: Int32(dataSize).littleEndian) { Array($0) })

    for i in 0..<samples {
        let t = Float(i) / sampleRate
        let envelope = max(0, 1.0 - t / duration)
        let sample = Int16(sin(2.0 * .pi * frequency * t) * Float(Int16.max) * envelope * 0.5)
        data.append(contentsOf: withUnsafeBytes(of: sample.littleEndian) { Array($0) })
    }

    return try? AVAudioPlayer(data: data)
}
```

Common game sounds:
- Hit/bounce: 440-880 Hz, 0.05-0.1s duration
- Score: 660 Hz then 880 Hz sequence, 0.1s each
- Game over: 220 Hz descending, 0.3s duration
- Serve/launch: 330 Hz rising, 0.15s duration

## Particle Effects

Create particles in code (no .sks files needed):

```swift
func createHitParticles(at position: CGPoint) -> SKEmitterNode {
    let emitter = SKEmitterNode()
    emitter.particleBirthRate = 200
    emitter.numParticlesToEmit = 20
    emitter.particleLifetime = 0.3
    emitter.particleSpeed = 150
    emitter.particleSpeedRange = 50
    emitter.emissionAngleRange = .pi * 2
    emitter.particleScale = 0.1
    emitter.particleScaleRange = 0.05
    emitter.particleAlphaSpeed = -3.0
    emitter.particleColor = .white
    emitter.position = position
    // Auto-remove after emission completes
    emitter.run(.sequence([.wait(forDuration: 0.5), .removeFromParent()]))
    return emitter
}
```

## Game Feel — Juice Effects

Screen shake on impact:
```swift
func screenShake(intensity: CGFloat = 8, duration: TimeInterval = 0.15) {
    let shake = SKAction.sequence([
        SKAction.moveBy(x: intensity, y: intensity, duration: duration / 4),
        SKAction.moveBy(x: -intensity * 2, y: -intensity, duration: duration / 4),
        SKAction.moveBy(x: intensity, y: -intensity, duration: duration / 4),
        SKAction.moveBy(x: 0, y: intensity, duration: duration / 4),
    ])
    camera?.run(shake)
}
```

Hit pause (freeze frame) — 50-100ms pause on impact:
```swift
func hitPause(duration: TimeInterval = 0.05) {
    isPaused = true
    DispatchQueue.main.asyncAfter(deadline: .now() + duration) { self.isPaused = false }
}
```

Haptic feedback:
```swift
let impact = UIImpactFeedbackGenerator(style: .medium)
impact.impactOccurred()
```

## Performance Rules

- **SKSpriteNode** for all game objects — SKShapeNode is slow (drops FPS significantly)
- Convert shapes to textures: `let texture = view.texture(from: shapeNode)`
- Preload textures before gameplay to avoid frame drops
- Use texture atlases for sprite sheets (group by scene/level, not one giant atlas)
- Verify scene deallocation: override `deinit` during development
- Static bodies for walls/platforms, dynamic only for moving objects
- Disable physics on off-screen nodes

## Genre-Specific Patterns

**Sports/Pong**: `gravity = .zero`, rectangle paddles, ball with `restitution = 1.0`, AI tracks ball with delay
**Platformer**: `gravity = CGVector(dx: 0, dy: -9.8)`, `applyImpulse()` for jumps, ground detection via contact
**Endless Runner**: Parallax scrolling with duplicate background sprites leap-frogging, SKCameraNode follows player
**Puzzle/Match-3**: Grid data structure, nested loop match detection, SKAction chains for animations
**Shooter**: Contact (not collision) tests for bullets, spawn timers for enemy waves, projectile pooling

## Game Assets

For visual assets (sprites, backgrounds, textures), load the `game-assets` skill to use the `nw_download_asset` tool. For sounds, generate programmatically as shown above.
