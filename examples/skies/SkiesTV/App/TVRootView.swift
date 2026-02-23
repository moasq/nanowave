import SwiftUI

struct TVRootView: View {
    @EnvironmentObject var store: WeatherStore
    var body: some View {
        TVMainView(snapshot: store.snapshot)
    }
}

#Preview {
    TVRootView()
        .environmentObject(WeatherStore.shared)
}
