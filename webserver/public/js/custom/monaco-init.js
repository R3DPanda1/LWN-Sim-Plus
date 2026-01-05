/**
 * Monaco Editor Initialization for Codec JavaScript Editor
 * Provides VS Code-like editing experience with custom autocomplete for helper functions
 */

"use strict";

// Global Monaco editor instance
window.codecEditor = null;

/**
 * Initialize Monaco Editor for the codec script textarea
 */
window.InitializeMonacoEditor = function() {
    if (window.codecEditor) {
        return; // Already initialized
    }

    const textarea = document.getElementById("textarea-codec-script");
    if (!textarea) {
        console.error("Monaco target textarea not found");
        return;
    }

    // Get initial value from textarea
    const initialValue = textarea.value || textarea.placeholder || "";

    // Create a container for Monaco
    const editorContainer = document.createElement("div");
    editorContainer.id = "monaco-editor-container";
    editorContainer.style.height = "800px";
    editorContainer.style.border = "1px solid #ced4da";
    editorContainer.style.borderRadius = "0.25rem";

    // Insert Monaco container after textarea
    textarea.parentNode.insertBefore(editorContainer, textarea.nextSibling);

    // Hide the original textarea
    textarea.style.display = "none";

    // Configure Monaco loader to use CDN
    require.config({
        paths: {
            'vs': 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.52.0/min/vs'
        }
    });

    // Load Monaco Editor
    require(['vs/editor/editor.main'], function() {
        // Register custom autocomplete provider for helper functions
        monaco.languages.registerCompletionItemProvider('javascript', {
            provideCompletionItems: function(model, position) {
                var suggestions = [
                    {
                        label: 'getState',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getState(${1:key})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Get a persistent state variable by key. Can store any JSON-serializable value.',
                        detail: '(key: string) => any'
                    },
                    {
                        label: 'setState',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'setState(${1:key}, ${2:value})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Set a persistent state variable. Value can be string, number, object, or array.',
                        detail: '(key: string, value: any) => void'
                    },
                    {
                        label: 'getSendInterval',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getSendInterval()',
                        documentation: 'Get the current device send interval in seconds.',
                        detail: '() => number'
                    },
                    {
                        label: 'setSendInterval',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'setSendInterval(${1:seconds})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Set the device send interval in seconds. Use to dynamically adjust uplink frequency.',
                        detail: '(seconds: number) => void'
                    },
                    {
                        label: 'hexToBytes',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'hexToBytes(${1:hexString})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Convert a hex string to byte array. Example: hexToBytes("48656C6C6F") returns [72, 101, 108, 108, 111]',
                        detail: '(hexString: string) => byte[]'
                    },
                    {
                        label: 'base64ToBytes',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'base64ToBytes(${1:b64String})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Convert a base64 string to byte array. Example: base64ToBytes("SGVsbG8=") returns [72, 101, 108, 108, 111]',
                        detail: '(b64String: string) => byte[]'
                    },
                    {
                        label: 'log',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'log(${1:message})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Log a debug message to the simulator console. Useful for troubleshooting codec logic.',
                        detail: '(message: string) => void'
                    },
                    // Add template suggestions for common codec patterns
                    {
                        label: 'OnUplink Template',
                        kind: monaco.languages.CompletionItemKind.Snippet,
                        insertText: [
                            'function OnUplink() {',
                            '    // Generate payload bytes for uplink',
                            '    var bytes = [];',
                            '',
                            '    // Your encoding logic here',
                            '    ${1:// Example: encode temperature}',
                            '    ${2:var temp = Math.round(22.5 * 10);}',
                            '    ${3:bytes.push((temp >> 8) & 0xFF);}',
                            '    ${4:bytes.push(temp & 0xFF);}',
                            '',
                            '    return {',
                            '        fPort: ${5:85},',
                            '        bytes: bytes',
                            '    };',
                            '}'
                        ].join('\n'),
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Template for OnUplink function',
                        detail: 'OnUplink function template'
                    },
                    {
                        label: 'OnDownlink Template',
                        kind: monaco.languages.CompletionItemKind.Snippet,
                        insertText: [
                            'function OnDownlink(bytes, fPort) {',
                            '    // Process downlink bytes (optional)',
                            '    // Use log() to output decoded values',
                            '',
                            '    ${1:// Example: decode temperature}',
                            '    ${2:var temp = (((bytes[0] << 8) | bytes[1]) << 16 >> 16) / 10.0;}',
                            '    ${3:log("Received temperature: " + temp);}',
                            '}'
                        ].join('\n'),
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Template for OnDownlink function',
                        detail: 'OnDownlink function template'
                    }
                ];
                return { suggestions: suggestions };
            }
        });

        // Create the Monaco editor
        window.codecEditor = monaco.editor.create(editorContainer, {
            value: initialValue,
            language: 'javascript',
            theme: 'vs',
            automaticLayout: true,
            minimap: {
                enabled: true
            },
            fontSize: 14,
            lineNumbers: 'on',
            roundedSelection: false,
            scrollBeyondLastLine: false,
            readOnly: false,
            tabSize: 4,
            insertSpaces: true,
            wordWrap: 'on',
            suggestOnTriggerCharacters: true,
            quickSuggestions: {
                other: true,
                comments: false,
                strings: false
            },
            parameterHints: {
                enabled: true
            },
            formatOnPaste: true,
            formatOnType: true
        });

        console.log("Monaco Editor initialized successfully");
    });
};

/**
 * Get the current value from Monaco editor
 */
window.GetMonacoEditorValue = function() {
    if (window.codecEditor) {
        return window.codecEditor.getValue();
    }
    return document.getElementById("textarea-codec-script").value;
};

/**
 * Set value in Monaco editor
 */
window.SetMonacoEditorValue = function(value) {
    if (window.codecEditor) {
        window.codecEditor.setValue(value || "");
    } else {
        document.getElementById("textarea-codec-script").value = value || "";
    }
};

/**
 * Set Monaco editor read-only mode
 */
window.SetMonacoEditorReadOnly = function(readOnly) {
    if (window.codecEditor) {
        window.codecEditor.updateOptions({ readOnly: readOnly });
    } else {
        document.getElementById("textarea-codec-script").disabled = readOnly;
    }
};

console.log("Monaco initialization module loaded");
