import SwiftUI

struct GlassCardModifier: ViewModifier {
    func body(content: Content) -> some View {
        content
            .padding(AppTheme.Spacing.md)
            .background(.ultraThinMaterial)
            .clipShape(.rect(cornerRadius: AppTheme.Style.cornerRadius))
            .overlay(
                RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius)
                    .stroke(.white.opacity(0.2), lineWidth: 0.5)
            )
    }
}

extension View {
    func glassCard() -> some View {
        modifier(GlassCardModifier())
    }
}

#Preview {
    ZStack {
        AppTheme.Colors.background.ignoresSafeArea()
        VStack {
            Text("Glass Card")
                .foregroundStyle(.white)
        }
        .glassCard()
    }
}
