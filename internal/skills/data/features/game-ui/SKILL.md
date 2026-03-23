---
name: "game-ui"
description: "Game UI patterns: SwiftUI HUD overlays on SpriteKit, menus (main/pause/game-over), virtual joystick/d-pad, score displays, health bars, tutorial onboarding."
---
# Game UI Patterns

## HUD Overlay Architecture

Use SwiftUI overlays on SpriteView — NOT SKLabelNode/SKSpriteNode for HUD:

```swift
struct GameView: View {
    @State var gameState = GameState()

    var body: some View {
        ZStack {
            SpriteView(scene: scene, isPaused: gameState.isPaused)
                .ignoresSafeArea()

            // HUD overlay — uses AppTheme
            VStack {
                HStack {
                    ScoreView(score: gameState.score)
                    Spacer()
                    LivesView(lives: gameState.lives)
                }
                .padding(.horizontal, AppTheme.Spacing.lg)
                .padding(.top, AppTheme.Spacing.md)

                Spacer()
            }
            .allowsHitTesting(false) // Let touches pass through to SpriteView

            // Pause/Game Over modals
            if gameState.phase == .paused { PauseMenuView(gameState: gameState) }
            if gameState.phase == .gameOver { GameOverView(gameState: gameState) }
        }
    }
}
```

Rules:
- HUD elements in SwiftUI use AppTheme tokens (colors, fonts, spacing)
- Use `.allowsHitTesting(false)` on non-interactive overlays
- Interactive buttons (pause, menu) need `.allowsHitTesting(true)`
- Score/lives/timer update via `@Observable` GameState shared with SKScene

## Menu Flow

```
MainMenu → Game (Playing) → Pause → Resume / Quit
                          → GameOver → PlayAgain / MainMenu
```

Implementation:
```swift
@Observable
class GameState {
    var phase: GamePhase = .menu

    enum GamePhase: Equatable {
        case menu
        case playing
        case paused
        case gameOver(winner: String)
    }
}

// In MainView
switch gameState.phase {
case .menu:
    MainMenuView(gameState: gameState)
case .playing, .paused, .gameOver:
    GameView(gameState: gameState)
}
```

## Score Display

```swift
struct ScoreView: View {
    let score: Int

    var body: some View {
        Text("\(score)")
            .font(AppTheme.Fonts.largeTitle)
            .foregroundStyle(AppTheme.Colors.textPrimary)
            .contentTransition(.numericText())
            .animation(.spring(duration: 0.3), value: score)
    }
}
```

## Virtual Joystick

For games needing continuous directional input (platformers, top-down):

```swift
struct JoystickView: View {
    @Binding var direction: CGVector
    @State private var knobOffset: CGSize = .zero
    let radius: CGFloat = 50

    var body: some View {
        ZStack {
            Circle()
                .fill(AppTheme.Colors.surface.opacity(0.3))
                .frame(width: radius * 2, height: radius * 2)
            Circle()
                .fill(AppTheme.Colors.primary.opacity(0.7))
                .frame(width: radius * 0.8, height: radius * 0.8)
                .offset(knobOffset)
        }
        .gesture(
            DragGesture()
                .onChanged { value in
                    let vector = CGSize(width: value.translation.width, height: value.translation.height)
                    let distance = sqrt(vector.width * vector.width + vector.height * vector.height)
                    if distance <= radius {
                        knobOffset = vector
                    } else {
                        knobOffset = CGSize(
                            width: vector.width / distance * radius,
                            height: vector.height / distance * radius
                        )
                    }
                    direction = CGVector(dx: knobOffset.width / radius, dy: -knobOffset.height / radius)
                }
                .onEnded { _ in
                    knobOffset = .zero
                    direction = .zero
                }
        )
    }
}
```

## Tutorial Onboarding

```swift
@AppStorage("hasCompletedTutorial") private var tutorialDone = false

// Show tutorial overlay on first launch
if !tutorialDone {
    TutorialOverlay(onComplete: { tutorialDone = true })
}
```

Pattern: highlight target area, show instruction text, require player action to advance.

## iPad Considerations

- Use `.scaleMode = .resizeFill` — scene adapts to any iPad size
- Test both portrait and landscape — use GeometryReader for responsive positioning
- Virtual controls: place joystick bottom-left, action buttons bottom-right
- Larger touch targets for iPad (minimum 60pt for game controls)
