// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import SwiftUI

struct ContentView: View {
    @EnvironmentObject private var server: ServerProcess
    @EnvironmentObject private var settings: AppSettings
    @State private var selection: SidebarDestination? = .overview
    @State private var columnVisibility: NavigationSplitViewVisibility = .all

    var body: some View {
        NavigationSplitView(columnVisibility: $columnVisibility) {
            List(selection: $selection) {
                Section("Decide") {
                    ForEach(SidebarDestination.primary) { item in
                        Label(item.title, systemImage: item.systemImage)
                            .tag(item)
                    }
                }
                Section("More") {
                    ForEach(SidebarDestination.more) { item in
                        Label(item.title, systemImage: item.systemImage)
                            .tag(item)
                    }
                }
            }
            .listStyle(.sidebar)
            .navigationTitle("Zyntra")
            .safeAreaInset(edge: .bottom) {
                serverFooter
                    .padding(12)
            }
        } detail: {
            detail
        }
        .navigationSplitViewStyle(.balanced)
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                statusChip
                Button {
                    Task { await server.restart(using: settings) }
                } label: {
                    Label("Restart", systemImage: "arrow.clockwise")
                }
                .help("Restart local server")

                Button {
                    server.openInBrowser()
                } label: {
                    Label("Browser", systemImage: "safari")
                }
                .disabled(!server.isReady)
                .help("Open console in browser")
            }
        }
        .task {
            if !server.isRunning {
                await server.start(using: settings)
            }
        }
        .onDisappear {
            // Keep server running while other windows/settings open; stop on app terminate via delegate.
        }
    }

    @ViewBuilder
    private var detail: some View {
        switch server.status {
        case .ready:
            if let base = server.baseURL {
                ConsoleWebView(url: base, destination: selection)
                    .ignoresSafeArea(edges: .bottom)
            } else {
                placeholder(title: "Ready", message: "Console URL missing.")
            }
        case .starting:
            placeholder(title: "Starting Zyntra", message: "Waiting for /healthz on localhost…")
        case .stopped:
            placeholder(title: "Server stopped", message: "Press Restart in the toolbar to start the local console.")
        case .failed(let message):
            placeholder(title: "Could not start", message: message, isError: true)
        }
    }

    private var statusChip: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
            Text(statusLabel)
                .font(.caption.weight(.medium))
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(.regularMaterial, in: Capsule())
    }

    private var statusColor: Color {
        switch server.status {
        case .ready: return .green
        case .starting: return .orange
        case .stopped: return .secondary
        case .failed: return .red
        }
    }

    private var statusLabel: String {
        switch server.status {
        case .ready: return "Local · \(server.port)"
        case .starting: return "Starting…"
        case .stopped: return "Stopped"
        case .failed: return "Error"
        }
    }

    private var serverFooter: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Local engine")
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            if !server.lastLogLine.isEmpty {
                Text(server.lastLogLine)
                    .font(.caption2.monospaced())
                    .foregroundStyle(.tertiary)
                    .lineLimit(2)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func placeholder(title: String, message: String, isError: Bool = false) -> some View {
        VStack(spacing: 16) {
            Image(systemName: isError ? "exclamationmark.triangle.fill" : "server.rack")
                .font(.system(size: 40))
                .foregroundStyle(isError ? Color.red.opacity(0.85) : Color.secondary)
            Text(title)
                .font(.title2.weight(.semibold))
            Text(message)
                .font(.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 420)
            if isError || server.status == .stopped {
                Button("Start Server") {
                    Task { await server.start(using: settings) }
                }
                .buttonStyle(.borderedProminent)
                .tint(Color(red: 1, green: 0.35, blue: 0.08))
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(.background)
    }
}
