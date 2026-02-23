import SwiftUI

struct RootView: View {
    @EnvironmentObject var store: WeatherStore
    var body: some View {
        MainView()
    }
}

#Preview {
    RootView()
        .environmentObject(WeatherStore.shared)
}
