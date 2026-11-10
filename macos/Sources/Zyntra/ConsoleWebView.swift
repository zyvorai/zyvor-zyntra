// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import AppKit
import SwiftUI
import WebKit

struct ConsoleWebView: NSViewRepresentable {
    let url: URL
    var destination: SidebarDestination?

    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    func makeNSView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()
        config.preferences.isElementFullscreenEnabled = true
        let view = WKWebView(frame: .zero, configuration: config)
        view.navigationDelegate = context.coordinator
        view.allowsBackForwardNavigationGestures = true
        view.setValue(false, forKey: "drawsBackground")
        context.coordinator.webView = view
        let initial = destination.flatMap { URL(string: $0.hashPath, relativeTo: url)?.absoluteURL } ?? url
        view.load(URLRequest(url: initial))
        context.coordinator.lastDestination = destination
        context.coordinator.baseURL = url
        return view
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        context.coordinator.webView = nsView
        let previous = context.coordinator.baseURL
        context.coordinator.baseURL = url

        if previous?.host != url.host || previous?.port != url.port {
            let initial = destination.flatMap { URL(string: $0.hashPath, relativeTo: url)?.absoluteURL } ?? url
            nsView.load(URLRequest(url: initial))
            context.coordinator.lastDestination = destination
            return
        }

        guard let destination, destination != context.coordinator.lastDestination else { return }
        context.coordinator.lastDestination = destination
        if nsView.url?.host == url.host, nsView.url?.port == url.port {
            nsView.evaluateJavaScript("window.location.hash = '/\(destination.rawValue)';", completionHandler: nil)
        } else if let destURL = URL(string: destination.hashPath, relativeTo: url)?.absoluteURL {
            nsView.load(URLRequest(url: destURL))
        }
    }

    final class Coordinator: NSObject, WKNavigationDelegate {
        weak var webView: WKWebView?
        var lastDestination: SidebarDestination?
        var baseURL: URL?

        func webView(
            _ webView: WKWebView,
            decidePolicyFor navigationAction: WKNavigationAction,
            decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
        ) {
            guard let requestURL = navigationAction.request.url else {
                decisionHandler(.cancel)
                return
            }
            if Self.isLocalConsole(requestURL) {
                decisionHandler(.allow)
                return
            }
            if navigationAction.navigationType == .linkActivated {
                NSWorkspace.shared.open(requestURL)
            }
            decisionHandler(.cancel)
        }

        private static func isLocalConsole(_ url: URL) -> Bool {
            let host = url.host?.lowercased() ?? ""
            guard host == "127.0.0.1" || host == "localhost" else { return false }
            let scheme = url.scheme?.lowercased() ?? ""
            return scheme == "http" || scheme == "https" || scheme == "about"
        }
    }
}
