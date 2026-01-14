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
        // Custom function type definitions for intellisense
        const customFunctionDefs = `
            /**
             * Called to generate a device uplink.
             * Must return an object with fPort and bytes.
             * @returns {{fPort: number, bytes: number[]}}
             */
            declare function OnUplink(): {fPort: number, bytes: number[]};

            /**
             * Called when a downlink is received for the device.
             * @param bytes The downlink payload bytes.
             * @param fPort The downlink fPort.
             */
            declare function OnDownlink(bytes: number[], fPort: number): void;

            /**
             * Get a persistent state variable by key.
             * Can store any JSON-serializable value.
             * @param key The key of the state variable.
             * @returns {any} The value of the state variable.
             */
            declare function getState(key: string): any;

            /**
             * Set a persistent state variable.
             * Value can be string, number, object, or array.
             * @param key The key of the state variable.
             * @param value The value to store.
             */
            declare function setState(key: string, value: any): void;

            /**
            * Get the current device send interval in seconds.
            * @returns {number} The send interval in seconds.
            */
            declare function getSendInterval(): number;

            /**
             * Set the device send interval in seconds.
             * Use to dynamically adjust uplink frequency.
             * @param seconds The new send interval in seconds.
             */
            declare function setSendInterval(seconds: number): void;

            /**
             * Convert a hex string to a byte array.
             * Example: hexToBytes("48656C6C6F") returns [72, 101, 108, 108, 111]
             * @param hexString The hex string to convert.
             * @returns {number[]} The resulting byte array.
             */
            declare function hexToBytes(hexString: string): number[];

            /**
             * Convert a base64 string to a byte array.
             * Example: base64ToBytes("SGVsbG8=") returns [72, 101, 108, 108, 111]
             * @param b64String The base64 string to convert.
             * @returns {number[]} The resulting byte array.
             */
            declare function base64ToBytes(b64String: string): number[];

            /**
             * Log a debug message to the simulator console.
             * Useful for troubleshooting codec logic.
             * @param message The message to log. It can be any object.
             */
            declare function log(message: any): void;
        `;

        // Add the custom function definitions to the JavaScript language service
        monaco.languages.typescript.javascriptDefaults.addExtraLib(customFunctionDefs, 'custom-lib.d.ts');


        // Register custom autocomplete provider for helper functions
        monaco.languages.registerCompletionItemProvider('javascript', {
            provideCompletionItems: function(model, position) {
                var suggestions = [
                    {
                        label: 'OnUplink',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'OnUplink()',
                        documentation: 'Called to generate a device uplink. Must return an object with fPort and bytes.',
                        detail: '() => { fPort: number, bytes: number[] }'
                    },
                    {
                        label: 'OnDownlink',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'OnDownlink(${1:bytes}, ${2:fPort})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Called when a downlink is received for the device.',
                        detail: '(bytes: number[], fPort: number) => void'
                    },
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
                        detail: '(hexString: string) => number[]'
                    },
                    {
                        label: 'base64ToBytes',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'base64ToBytes(${1:b64String})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Convert a base64 string to byte array. Example: base64ToBytes("SGVsbG8=") returns [72, 101, 108, 108, 111]',
                        detail: '(b64String: string) => number[]'
                    },
                    {
                        label: 'log',
                        kind: monaco.languages.CompletionItemKind.Function,
                        insertText: 'log(${1:message})',
                        insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                        documentation: 'Log a debug message to the simulator console. Useful for troubleshooting codec logic.',
                        detail: '(message: any) => void'
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

        //console.log("Monaco Editor initialized successfully");
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
