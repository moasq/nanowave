---
name: "scenekit-game"
description: "Use for 3D games: racing, 3D sports, board games, marble maze, tower defense, bowling. SceneKit + SceneView architecture, 3D scene hierarchy, physics, game loop, primitives, materials, cameras, particles, audio."
---
# SceneKit 3D Game Development

**ARCHITECTURE OVERRIDE**: 3D games use SceneKit (SCNScene + SceneView), NOT SpriteKit or plain SwiftUI views with timers or Canvas. The standard @main App → RootView → MainView pattern still applies, but MainView hosts a SceneView instead of standard SwiftUI content. AppTheme rules apply to SwiftUI parts (menus, overlays, HUD) but NOT to SceneKit scene internals where you use SCNVector3, SCNMaterial colors, etc. directly.

Also load the `game-assets` skill for downloading 3D models and textures via `nw_download_asset`.
Also load the `game-ui` skill for SwiftUI HUD overlays, menus, and virtual controls.

## SceneView — SwiftUI Bridge

```swift
import SceneKit
import SwiftUI

struct GameView: View {
    @State var gameState = GameState()
    @State private var scene = GameScene()

    var body: some View {
        ZStack {
            SceneView(
                scene: scene.scnScene,
                pointOfView: scene.cameraNode,
                options: [.allowsCameraControl, .autoenablesDefaultLighting]
            )
            .ignoresSafeArea()

            // SwiftUI overlays for HUD, menus (use AppTheme here)
            if gameState.phase == .paused {
                PauseMenuView(gameState: gameState)
            }
        }
        .onAppear { scene.gameState = gameState }
    }
}
```

Rules:
- Use `SceneView(scene:pointOfView:options:)` — the native SwiftUI bridge for SceneKit
- `.allowsCameraControl` enables orbit/pan/zoom (remove for fixed-camera games)
- `.autoenablesDefaultLighting` adds ambient + directional light (remove when adding custom lights)
- Pass data between SwiftUI and SceneKit via `@Observable` game state objects
- SwiftUI overlays (menus, HUD, pause) sit in a ZStack above SceneView
- SwiftUI parts use AppTheme. SceneKit scene internals use SCNMaterial/SCNVector3 directly.

## Scene Architecture

```swift
@Observable
class GameScene: NSObject, SCNSceneRendererDelegate {
    let scnScene = SCNScene()
    let cameraNode = SCNNode()
    var gameState: GameState?

    override init() {
        super.init()
        setupCamera()
        setupLighting()
        setupEnvironment()
    }

    private func setupCamera() {
        cameraNode.camera = SCNCamera()
        cameraNode.position = SCNVector3(0, 10, 15)
        cameraNode.look(at: SCNVector3Zero)
        scnScene.rootNode.addChildNode(cameraNode)
    }

    private func setupLighting() {
        let ambientLight = SCNNode()
        ambientLight.light = SCNLight()
        ambientLight.light?.type = .ambient
        ambientLight.light?.intensity = 500
        scnScene.rootNode.addChildNode(ambientLight)

        let directionalLight = SCNNode()
        directionalLight.light = SCNLight()
        directionalLight.light?.type = .directional
        directionalLight.light?.intensity = 1000
        directionalLight.light?.castsShadow = true
        directionalLight.eulerAngles = SCNVector3(-Float.pi / 4, 0, 0)
        scnScene.rootNode.addChildNode(directionalLight)
    }
}
```

Node hierarchy — organize by purpose:
```
SCNScene.rootNode
├── environmentNode  — floor, skybox, static scenery
├── gameplayNode     — player, enemies, projectiles, pickups
├── cameraNode       — camera + attached HUD elements
└── lightingNode     — ambient, directional, spot lights
```

Rules:
- Group nodes under parent SCNNode containers for organization
- Use `SCNNode.addChildNode()` to build the hierarchy
- Camera: create an SCNCamera, attach to an SCNNode, set `scnScene.rootNode.addChildNode()`
- Lighting: always add ambient + directional lights. Use `castsShadow = true` for key light.

## Built-in 3D Primitives

Build complete games from SceneKit primitives — no external 3D models needed:

```swift
// Box (crates, buildings, platforms)
let box = SCNBox(width: 1, height: 1, length: 1, chamferRadius: 0.05)

// Sphere (balls, planets, projectiles)
let sphere = SCNSphere(radius: 0.5)

// Cylinder (pins, pillars, coins)
let cylinder = SCNCylinder(radius: 0.3, height: 1.5)

// Cone (trees, rockets, markers)
let cone = SCNCone(topRadius: 0, bottomRadius: 0.5, height: 1)

// Torus (rings, donuts, orbits)
let torus = SCNTorus(ringRadius: 1, pipeRadius: 0.2)

// Plane (walls, cards, billboards)
let plane = SCNPlane(width: 2, height: 2)

// Floor (infinite ground plane with reflections)
let floor = SCNFloor()
floor.reflectivity = 0.2

// Tube (hollow cylinder, pipes)
let tube = SCNTube(innerRadius: 0.4, outerRadius: 0.5, height: 1)

// Capsule (characters, rounded obstacles)
let capsule = SCNCapsule(capRadius: 0.3, height: 1.5)

// Pyramid (obstacles, decorations)
let pyramid = SCNPyramid(width: 1, height: 1.5, length: 1)

// Text (3D text labels in the scene)
let text = SCNText(string: "GOAL!", extrusionDepth: 0.2)
text.font = UIFont.systemFont(ofSize: 1.0, weight: .bold)
```

Creating a node from geometry:
```swift
let node = SCNNode(geometry: box)
node.position = SCNVector3(0, 0.5, 0)
scnScene.rootNode.addChildNode(node)
```

## Materials

```swift
let material = SCNMaterial()
material.diffuse.contents = UIColor.systemBlue      // Base color
material.specular.contents = UIColor.white           // Shiny highlights
material.roughness.contents = 0.4                    // 0 = mirror, 1 = matte
material.metalness.contents = 0.1                    // 0 = plastic, 1 = metal

let node = SCNNode(geometry: sphere)
node.geometry?.firstMaterial = material
```

Programmatic textures for materials:
```swift
func checkerboardTexture(size: CGFloat, colors: (UIColor, UIColor)) -> UIImage {
    let renderer = UIGraphicsImageRenderer(size: CGSize(width: size, height: size))
    return renderer.image { ctx in
        let half = size / 2
        colors.0.setFill()
        ctx.fill(CGRect(x: 0, y: 0, width: half, height: half))
        ctx.fill(CGRect(x: half, y: half, width: half, height: half))
        colors.1.setFill()
        ctx.fill(CGRect(x: half, y: 0, width: half, height: half))
        ctx.fill(CGRect(x: 0, y: half, width: half, height: half))
    }
}

// Apply to floor
floor.firstMaterial?.diffuse.contents = checkerboardTexture(size: 256, colors: (.darkGray, .gray))
```

Multi-material per geometry:
```swift
let box = SCNBox(width: 1, height: 1, length: 1, chamferRadius: 0)
box.materials = [frontMat, rightMat, backMat, leftMat, topMat, bottomMat]
```

## Physics

```swift
struct PhysicsCategory {
    static let none:    Int = 0
    static let player:  Int = 1 << 0
    static let enemy:   Int = 1 << 1
    static let ball:    Int = 1 << 2
    static let floor:   Int = 1 << 3
    static let wall:    Int = 1 << 4
}

// Dynamic body (moves, affected by gravity)
node.physicsBody = SCNPhysicsBody(type: .dynamic, shape: SCNPhysicsShape(geometry: sphere, options: nil))
node.physicsBody?.mass = 1.0
node.physicsBody?.restitution = 0.8  // Bounciness
node.physicsBody?.friction = 0.5
node.physicsBody?.categoryBitMask = PhysicsCategory.ball
node.physicsBody?.contactTestBitMask = PhysicsCategory.floor | PhysicsCategory.enemy
node.physicsBody?.collisionBitMask = PhysicsCategory.floor | PhysicsCategory.wall

// Static body (immovable — floors, walls)
floorNode.physicsBody = SCNPhysicsBody(type: .static, shape: nil)
floorNode.physicsBody?.categoryBitMask = PhysicsCategory.floor

// Kinematic body (moved by code, not gravity — moving platforms)
platform.physicsBody = SCNPhysicsBody(type: .kinematic, shape: nil)
```

Contact detection:
```swift
extension GameScene: SCNPhysicsContactDelegate {
    func physicsWorld(_ world: SCNPhysicsWorld, didBegin contact: SCNPhysicsContact) {
        let nodeA = contact.nodeA
        let nodeB = contact.nodeB
        // Handle collision based on category bitmasks
    }
}

// Set delegate
scnScene.physicsWorld.contactDelegate = self
```

## Game Loop

```swift
extension GameScene: SCNSceneRendererDelegate {
    private var lastUpdateTime: TimeInterval { get set }  // Store as property

    func renderer(_ renderer: SCNSceneRenderer, updateAtTime time: TimeInterval) {
        let dt = lastUpdateTime == 0 ? 0 : time - lastUpdateTime
        lastUpdateTime = time

        updateGameLogic(deltaTime: dt)
    }
}
```

Wire the delegate in SceneView:
```swift
SceneView(
    scene: scene.scnScene,
    pointOfView: scene.cameraNode,
    options: [.allowsCameraControl],
    delegate: scene  // SCNSceneRendererDelegate
)
```

Rules:
- Always calculate delta time — never assume fixed frame rate
- `renderer(_:updateAtTime:)` runs every frame — keep it lightweight
- Use `renderer(_:didApplyAnimations:)` for post-animation logic
- Use `renderer(_:didSimulatePhysics:)` for post-physics corrections

## Game State with @Observable

```swift
@Observable
class GameState {
    var score: Int = 0
    var lives: Int = 3
    var isPaused: Bool = false
    var phase: GamePhase = .menu

    enum GamePhase: Equatable {
        case menu, playing, paused, gameOver
    }
}
```

Share between SwiftUI and SCNScene — scene updates properties, SwiftUI reacts automatically.

## Camera Systems

Third-person follow camera:
```swift
func updateCamera(following target: SCNNode) {
    let offset = SCNVector3(0, 5, 10)
    let targetPos = SCNVector3(
        target.position.x + offset.x,
        target.position.y + offset.y,
        target.position.z + offset.z
    )
    cameraNode.position = SCNVector3(
        cameraNode.position.x + (targetPos.x - cameraNode.position.x) * 0.1,
        cameraNode.position.y + (targetPos.y - cameraNode.position.y) * 0.1,
        cameraNode.position.z + (targetPos.z - cameraNode.position.z) * 0.1
    )
    cameraNode.look(at: target.position)
}
```

Fixed overhead camera (board games, tower defense):
```swift
cameraNode.position = SCNVector3(0, 20, 0)
cameraNode.eulerAngles = SCNVector3(-Float.pi / 2, 0, 0)
```

## Animations

SCNAction (like SKAction for 3D):
```swift
// Move
let move = SCNAction.move(to: SCNVector3(0, 2, 0), duration: 0.5)

// Rotate
let rotate = SCNAction.rotateBy(x: 0, y: .pi * 2, z: 0, duration: 1.0)

// Scale
let scale = SCNAction.scale(to: 1.5, duration: 0.3)

// Sequence and group
let bounceUp = SCNAction.moveBy(x: 0, y: 1, z: 0, duration: 0.2)
let bounceDown = bounceUp.reversed()
let bounce = SCNAction.sequence([bounceUp, bounceDown])
let spinAndBounce = SCNAction.group([rotate, bounce])

// Repeat
node.runAction(.repeatForever(rotate))

// Run with completion
node.runAction(move) {
    // Done
}
```

SCNTransaction for implicit animations:
```swift
SCNTransaction.begin()
SCNTransaction.animationDuration = 0.5
node.position = SCNVector3(5, 0, 0)
node.opacity = 0.5
SCNTransaction.commit()
```

## Particle Effects

```swift
func createExplosion(at position: SCNVector3) -> SCNParticleSystem {
    let particles = SCNParticleSystem()
    particles.birthRate = 500
    particles.emissionDuration = 0.1
    particles.particleLifeSpan = 0.5
    particles.spreadingAngle = 180
    particles.particleSize = 0.1
    particles.particleColor = .orange
    particles.particleColorVariation = SCNVector4(0.2, 0.2, 0, 0)
    particles.particleVelocity = 5
    particles.particleVelocityVariation = 2
    particles.isAffectedByGravity = true

    let particleNode = SCNNode()
    particleNode.position = position
    particleNode.addParticleSystem(particles)

    // Auto-remove after emission
    particleNode.runAction(.sequence([
        .wait(duration: 1.0),
        .removeFromParentNode()
    ]))

    return particles
}
```

## Audio

Positional 3D audio:
```swift
let audioSource = SCNAudioSource(named: "engine.wav")!
audioSource.isPositional = true
audioSource.shouldStream = false
audioSource.load()

let audioPlayer = SCNAudioPlayer(source: audioSource)
engineNode.addAudioPlayer(audioPlayer)
```

For procedural sound generation, use the same `generateTone()` pattern from the `spritekit-game` skill with AVAudioPlayer.

## Game Feel

Camera shake:
```swift
func cameraShake(intensity: Float = 0.3, duration: TimeInterval = 0.15) {
    let shake = SCNAction.sequence([
        .moveBy(x: CGFloat(intensity), y: CGFloat(intensity), z: 0, duration: duration / 4),
        .moveBy(x: CGFloat(-intensity * 2), y: CGFloat(-intensity), z: 0, duration: duration / 4),
        .moveBy(x: CGFloat(intensity), y: CGFloat(-intensity), z: 0, duration: duration / 4),
        .moveBy(x: 0, y: CGFloat(intensity), z: 0, duration: duration / 4),
    ])
    cameraNode.runAction(shake)
}
```

Haptic feedback:
```swift
let impact = UIImpactFeedbackGenerator(style: .medium)
impact.impactOccurred()
```

## 3D Model Loading

Load USDZ/OBJ/DAE/SCN files from the app bundle:
```swift
// .scn or .dae files
let scene = SCNScene(named: "model.scn")!
let modelNode = scene.rootNode.childNodes.first!

// USDZ via ModelIO
import ModelIO
import SceneKit.ModelIO

let url = Bundle.main.url(forResource: "model", withExtension: "usdz")!
let mdlAsset = MDLAsset(url: url)
let modelScene = SCNScene(mdlAsset: mdlAsset)
```

Use `nw_download_asset` with `asset_kind: "model"` to download 3D models into the project's Models/ directory.

## Performance Rules

- Use simple physics shapes (box, sphere) instead of mesh-based shapes
- `flattenedClone()` for static geometry groups — merges into single draw call
- Reuse SCNGeometry and SCNMaterial instances across nodes
- Use `SCNNode.isHidden = true` for off-screen nodes (skips rendering)
- Limit shadow-casting lights (1-2 max)
- Use `SCNCamera.fieldOfView` to control visible area (smaller FOV = less to render)
- Profile with Xcode's SceneKit statistics overlay: `sceneView.showsStatistics = true`

## Genre-Specific Patterns

**Bowling/Sports**: Lane as SCNFloor, pins as SCNCylinder, ball as SCNSphere with `.dynamic` physics, `applyForce()` for throw
**Racing**: SCNPhysicsVehicle for car physics, SCNFloor for track, checkpoint nodes with contact detection
**Board Games**: SCNPlane/SCNBox tiles, overhead fixed camera, tap gestures via `hitTest()` for piece selection
**Marble Maze**: SCNCapsule/SCNSphere player, SCNBox platforms, tilt controls via CoreMotion accelerometer
**Tower Defense**: Grid-based SCNPlane tiles, SCNBox/SCNCylinder towers, pathfinding for enemies, projectile SCNSphere nodes
