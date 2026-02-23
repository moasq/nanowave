import SwiftUI

struct TVMainView: View {
    let snapshot: WeatherSnapshot
    @State private var selectedTab = 0
    var body: some View {
        TabView(selection: $selectedTab) {
            Tab("Conditions", systemImage: "cloud.sun.fill", value: 0) {
                TVConditionsTabView(current: snapshot.current)
            }
            Tab("Hourly", systemImage: "clock.fill", value: 1) {
                ZStack {
                    conditionBackground
                    TVHourlyTabView(hourly: snapshot.hourly)
                }
            }
            Tab("7-Day", systemImage: "calendar", value: 2) {
                ZStack {
                    conditionBackground
                    TVDailyTabView(daily: snapshot.daily)
                }
            }
        }
    }
    private var conditionBackground: some View {
        LinearGradient(
            colors: snapshot.current.condition.gradientColors,
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .ignoresSafeArea()
    }
}

#Preview {
    TVMainView(snapshot: .sampleData)
}
