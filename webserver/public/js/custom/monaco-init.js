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
    editorContainer.style.height = "500px";
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
                        label: 'getCounter',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getCounter(${1:name})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Get a persistent counter value by name. The counter persists across device uplinks.',
                        detail: '(name: string) => number'
                    },
                    {
                        label: 'setCounter',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'setCounter(${1:name}, ${2:value})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Set a persistent counter value. Useful for tracking sequences or incrementing values.',
                        detail: '(name: string, value: number) => void'
                    },
                    {
                        label: 'getState',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getState(${1:name})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Get a persistent state variable by name. Can store any JSON-serializable value.',
                        detail: '(name: string) => any'
                    },
                    {
                        label: 'setState',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'setState(${1:name}, ${2:value})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Set a persistent state variable. Value can be string, number, object, or array.',
                        detail: '(name: string, value: any) => void'
                    },
                    {
                        label: 'getPreviousPayload',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getPreviousPayload()',
                        documentation: 'Get the previous payload bytes sent by this device. Returns null if no previous payload exists.',
                        detail: '() => byte[] | null'
                    },
                    {
                        label: 'getPreviousPayloads',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'getPreviousPayloads(${1:n})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Get the last n payload bytes sent by this device. Returns an array of byte arrays.',
                        detail: '(n: number) => byte[][]'
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
                        label: 'Encode Template',
                        kind: monaco.languages.CompletionItemKind.Snippet,
                        insertText: [
                            'function Encode(fPort, obj) {',
                            '    // Generate payload bytes from JSON object',
                            '    var bytes = [];',
                            '',
                            '    // Your encoding logic here',
                            '    ${1:// Example: encode temperature}',
                            '    ${2:var temp = Math.round((obj.temperature || 20) * 10);}',
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
                        documentation: 'Template for Encode function',
                        detail: 'Encode function template'
                    },
                    {
                        label: 'Decode Template',
                        kind: monaco.languages.CompletionItemKind.Snippet,
                        insertText: [
                            'function Decode(fPort, bytes) {',
                            '    // Decode bytes to JSON object',
                            '    var obj = {};',
                            '',
                            '    // Your decoding logic here',
                            '    ${1:// Example: decode temperature}',
                            '    ${2:obj.temperature = (((bytes[0] << 8) | bytes[1]) << 16 >> 16) / 10.0;}',
                            '',
                            '    return obj;',
                            '}'
                        ].join('\n'),
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Template for Decode function',
                        detail: 'Decode function template'
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
