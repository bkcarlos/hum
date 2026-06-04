import SwiftUI

/// BYOK 设置：provider 预设 / Base URL / 模型（可拉取）/ API Key / 测试连接 / 清除。
struct SettingsView: View {
    @EnvironmentObject private var llm: LLMConfigStore
    @Environment(\.dismiss) private var dismiss

    @State private var testing = false
    @State private var testMsg = ""
    @State private var testOk: Bool?
    @State private var loadingModels = false
    @State private var modelsMsg = ""
    @State private var modelsOk: Bool?

    var body: some View {
        NavigationStack {
            Form {
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

                Section {
                    Button("清除本地配置", role: .destructive) { llm.wipe(); resetHints() }
                }
            }
            .navigationTitle("LLM 设置 · BYOK")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("完成") { llm.persist(); dismiss() }
                }
            }
            .onDisappear { llm.persist() }
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
