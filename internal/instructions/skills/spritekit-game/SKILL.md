---
name: "spritekit-game"
description: "SpriteKit 2D game development: SpriteView integration, scene architecture, entity-component system, physics, game loop. Use when building 2D games."
---
# SpriteKit Game Development

## SpriteView — SwiftUI Bridge

SpriteView is the bridge between SwiftUI and SpriteKit. It embeds an SKScene inside a SwiftUI view hierarchy.

```swift
import SpriteKit
import SwiftUI

struct GameView: View {
    var body: some View {
        SpriteView(scene: GameScene(size: CGSize(width: 390, height: 844)))
            .ignoresSafeArea()
    }
}
```

Key rules:
- One `SpriteView` per game screen — it owns and manages the SKScene lifecycle
- Use `SpriteView(scene:, transition:, isPaused:, preferredFramesPerSecond:, options:, debugOptions:)` for full control
- SwiftUI views (menus, settings, HUD overlays) still use MVVM + AppTheme
- Game screens use SKScene architecture (not MVVM)
- Pass data between SwiftUI and SpriteKit via `@Observable` game state objects

## Scene Architecture

Every game scene uses a layer-based node hierarchy:

```
SKScene (GameScene)
├── backgroundLayer (SKNode) — z: -100, parallax backgrounds, sky
├── gameplayLayer (SKNode)  — z: 0, player, enemies, items, platforms
└── hudLayer (SKNode)       — z: 100, score, health bars, controls
```

Rules:
- Create layer nodes in `didMove(to:)`, add all game objects as children of the appropriate layer
- Use `zPosition` on layers, not on individual sprites
- Camera: attach `SKCameraNode` to the gameplay layer, set `scene.camera`
- Scene transitions: `SKTransition` for moving between scenes

## Entity-Component System (GameplayKit)

Use GKEntity + GKComponent for game object composition:

```swift
import GameplayKit

class PlayerEntity: GKEntity {
    init(spriteNode: SKSpriteNode) {
        super.init()
        addComponent(SpriteComponent(node: spriteNode))
        addComponent(HealthComponent(maxHealth: 100))
        addComponent(MovementComponent(speed: 200))
    }
}
```

Rules:
- Prefer composition (components) over inheritance (subclassing SKSpriteNode)
- GKComponentSystem updates all components of a type in the game loop
- Use GKStateMachine for game states (Menu, Playing, Paused, GameOver)

## Physics System

SKPhysicsBody with category bitmasks for collision detection:

```swift
struct PhysicsCategory {
    static let player:  UInt32 = 0x1 << 0
    static let enemy:   UInt32 = 0x1 << 1
    static let item:    UInt32 = 0x1 << 2
    static let ground:  UInt32 = 0x1 << 3
}
```

Rules:
- Set `categoryBitMask`, `contactTestBitMask`, `collisionBitMask` on every physics body
- Implement `SKPhysicsContactDelegate` on the scene for contact callbacks
- Use `physicsWorld.gravity` for platformers, set to `.zero` for top-down games

## Game Loop

The update cycle runs every frame via `update(_:)`:

```swift
override func update(_ currentTime: TimeInterval) {
    let dt = currentTime - lastUpdateTime
    lastUpdateTime = currentTime
    componentSystem.update(deltaTime: dt)
}
```

Rules:
- Always calculate delta time — never assume fixed frame rate
- Use `update()` for game logic, `didEvaluateActions()` for post-action cleanup
- `didSimulatePhysics()` for post-physics position corrections

## References

Detailed references are available for each subsystem:
- Scene architecture and lifecycle
- SpriteView SwiftUI integration
- Physics and collisions
- Entity-Component system
- Actions and animations
- Particle effects
- Tile maps
- Audio
- Performance optimization
