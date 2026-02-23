import SwiftUI

struct TVCloudParallaxView: View {
    var body: some View {
        TimelineView(.animation) { timeline in
            Canvas { context, size in
                let time = timeline.date.timeIntervalSince1970
                drawCloudLayer(context: context, size: size, time: time, speed: 0.03, yRatio: 0.25, opacity: 0.08, count: 5)
                drawCloudLayer(context: context, size: size, time: time, speed: 0.05, yRatio: 0.5, opacity: 0.12, count: 4)
                drawCloudLayer(context: context, size: size, time: time, speed: 0.08, yRatio: 0.7, opacity: 0.18, count: 3)
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
    private func drawCloudLayer(context: GraphicsContext, size: CGSize, time: Double, speed: Double, yRatio: Double, opacity: Double, count: Int) {
        for i in 0..<count {
            let fraction = Double(i) / Double(count)
            let xOffset = sin(time * speed + fraction * .pi * 2) * size.width * 0.15
            let x = fraction * size.width + xOffset
            let y = yRatio * size.height + sin(time * speed * 0.7 + fraction * 3) * 30
            let w = size.width * 0.25
            let h = w * 0.35
            let rect = CGRect(x: x - w / 2, y: y - h / 2, width: w, height: h)
            let ellipse = Path(ellipseIn: rect)
            context.fill(ellipse, with: .color(.white.opacity(opacity)))
        }
    }
}

#Preview {
    ZStack {
        Color(hex: "0D1B2A").ignoresSafeArea()
        TVCloudParallaxView()
    }
}
