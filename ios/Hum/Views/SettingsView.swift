import SwiftUI

/// 接入设置：免费额度（Sign in with Apple）/ 自带 Key（BYOK）二选一。
/// 顶部分段切换；免费档段 = 登录/退出；自带 Key 段 = provider/key/baseUrl/模型/测试。
struct SettingsView: View {
    @EnvironmentObject private var llm: LLMConfigStore
    @EnvironmentObject private var session: SessionStore
    @Environment(\.dismiss) private var dismiss

    @State private var testing = false
    @State private var testMsg = ""
    @State private var testOk: Bool?
    @State private var loadingModels = false
    @State private var modelsMsg = ""
    @State private var modelsOk: Bool?
    @State private var showClearConfirm = false

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Picker("接入方式", selection: $session.mode) {
                        Text("免费额度").tag(SessionStore.Mode.free)
                        Text("自带 Key").tag(SessionStore.Mode.byok)
                    }
                    .pickerStyle(.segmented)
                } footer: {
                    Text(session.mode == .free
                         ? "用 Apple 登录即可，每天有固定免费次数，无需自备 key。"
                         : "用你自己的 LLM Key，不限量；Key 只存本机钥匙串、用完即弃。")
                }

                if session.mode == .free {
                    freeSection
                } else {
                    byokSections
                }

                Section {
                    NavigationLink {
                        DiagnosticsView()
                    } label: {
                        Label("诊断日志", systemImage: "ladybug")
                    }
                } footer: {
                    Text("出问题时在这里查看、复制或分享最近的请求日志（不含任何密钥/令牌）。")
                }
            }
            .navigationTitle("接入设置")
            .navigationBarTitleDisplayMode(.inline)
            .scrollDismissesKeyboard(.interactively)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    if session.mode == .byok {
                        Button("清除", role: .destructive) { showClearConfirm = true }
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button("完成") { llm.persist(); dismiss() }
                }
            }
            .confirmationDialog("清除本地配置？", isPresented: $showClearConfirm, titleVisibility: .visible) {
                Button("清除并恢复默认", role: .destructive) { llm.wipe(); resetHints() }
                Button("取消", role: .cancel) {}
            } message: {
                Text("将删除本机保存的 Key 与自定义配置。")
            }
            .onDisappear { llm.persist() }
        }
    }

    // MARK: - 免费额度（Sign in with Apple）

    @ViewBuilder private var freeSection: some View {
        if session.signedIn {
            Section {
                if !session.email.isEmpty {
                    HStack {
                        Text("Apple ID")
                        Spacer()
                        Text(session.email).foregroundStyle(.secondary).lineLimit(1)
                    }
                }
                Button("退出登录", role: .destructive) { session.signOut() }
            } header: {
                Text("已登录")
            } footer: {
                Text("出推荐用的是服务端的 LLM Key，你无需任何配置。会话令牌只存本机钥匙串、不同步 iCloud。退出登录不影响你的自带 Key。")
            }
        } else {
            Section {
                AppleSignInButton()
                    .listRowInsets(EdgeInsets(top: 8, leading: 16, bottom: 8, trailing: 16))
                if session.loggingIn {
                    HStack(spacing: 6) {
                        ProgressView()
                        Text("登录中…").font(.caption).foregroundStyle(.secondary)
                    }
                }
                if !session.loginError.isEmpty {
                    Text(session.loginError).font(.caption).foregroundStyle(.red)
                }
            } header: {
                Text("免费额度")
            } footer: {
                Text("用 Apple 登录即可免费试用：每天有固定次数、无需自备 key。不收费、不读取你的资料库，只保存一个本机会话令牌。没订阅 Apple Music 也能出推荐 + 30s 试听。")
            }
        }
    }

    // MARK: - 自带 Key（BYOK）

    @ViewBuilder private var byokSections: some View {
        Section("Provider") {
            Picker("服务商", selection: Binding(
                get: { llm.presetId },
                set: { llm.applyPreset($0); resetHints() }
            )) {
                ForEach(LLMConfigStore.presets) { Text($0.label).tag($0.id) }
            }
        }

        Section("API Key") {
            SecureField("粘贴你的 API Key", text: $llm.apiKey)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .onChange(of: llm.apiKey) { _ in resetHints() }
        }

        Section("Base URL") {
            TextField("https://…", text: $llm.baseUrl)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .keyboardType(.URL)
                .onChange(of: llm.baseUrl) { _ in resetHints() }
        }

        Section("模型") {
            if !llm.modelOptions.isEmpty {
                Picker("选择模型", selection: $llm.model) {
                    if !llm.model.isEmpty && !llm.modelOptions.contains(llm.model) {
                        Text(llm.model).tag(llm.model)
                    }
                    ForEach(llm.modelOptions, id: \.self) { Text($0).tag($0) }
                }
            }
            TextField("模型名（可手动输入）", text: $llm.model)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
            Button {
                Task { await fetchModels() }
            } label: {
                HStack { Text("拉取模型"); if loadingModels { ProgressView() } }
            }
            .disabled(!canFetch || loadingModels)
            if let modelsOk {
                Text(modelsMsg).font(.caption).foregroundStyle(modelsOk ? .green : .red)
            }
        }

        Section {
            Button {
                Task { await runTest() }
            } label: {
                HStack { Text("测试连接"); if testing { ProgressView() } }
            }
            .disabled(!llm.configured || testing)
            if let testOk {
                Text(testMsg).font(.caption).foregroundStyle(testOk ? .green : .red)
            }
        } footer: {
            Text("你的 API Key 只保存在本机钥匙串，调用时随请求转发给后端用于本次 LLM 调用、用完即弃；不在服务器保存、不记录日志。请勿在公共设备上保存。")
        }
    }

    private var canFetch: Bool { !llm.apiKey.trimmed.isEmpty && !llm.baseUrl.trimmed.isEmpty }

    private func resetHints() {
        testOk = nil; testMsg = ""; modelsOk = nil; modelsMsg = ""
    }

    private func runTest() async {
        testing = true; testOk = nil; testMsg = ""
        switch await llm.test() {
        case .success: testOk = true; testMsg = "连接成功，Key 有效。"
        case .failure(let e): testOk = false; testMsg = e.userMessage
        }
        testing = false
    }

    private func fetchModels() async {
        loadingModels = true; modelsOk = nil; modelsMsg = ""
        switch await llm.fetchModels() {
        case .success(let n):
            modelsOk = true
            modelsMsg = n > 0 ? "已拉取 \(n) 个模型。" : "该服务商未返回模型列表，请手动输入。"
        case .failure(let e):
            modelsOk = false
            modelsMsg = "拉取失败：\(e.userMessage)（可手动输入）"
        }
        loadingModels = false
    }
}
