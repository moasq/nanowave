import SwiftUI

struct ConditionBackgroundView: View {
    let condition: WeatherCondition
    var body: some View {
        LinearGradient(
            colors: condition.gradientColors,
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .ignoresSafeArea()
        .animation(.easeInOut(duration: 1.2), value: condition)
    }
}

#Preview {
    ConditionBackgroundView(condition: .sunny)
}
