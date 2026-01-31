import { chmodSync, existsSync, readdirSync, statSync } from "fs";
import * as path from "path";
import {
  workspace,
  ExtensionContext,
  window,
  languages,
  CompletionItem as VCompletionItem,
  CompletionList,
  CompletionItemKind as VCompletionItemKind,
  Hover as VHover,
  MarkdownString,
  Range as VRange,
  Position as VPosition,
  SnippetString,
  TextDocument as VTextDocument,
  CancellationToken,
  CompletionContext,
} from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  Executable,
  TransportKind,
} from "vscode-languageclient/node";
import {
  getLanguageService as getHTMLLanguageService,
  TextDocument as LSPTextDocument,
  CompletionItemKind as LSPCompletionItemKind,
  InsertTextFormat,
  LanguageService as HTMLLanguageService,
} from "vscode-html-languageservice";

let client: LanguageClient | undefined;

function stripTemplateActions(text: string): string {
  return text.replace(/\{\{-?[\s\S]*?-?\}\}/g, (m) => " ".repeat(m.length));
}

function isInsideTemplateAction(lineText: string, charPos: number): boolean {
  let depth = 0;
  for (let i = 0; i < charPos && i < lineText.length; i++) {
    if (lineText[i] === "{" && i + 1 < lineText.length && lineText[i + 1] === "{") {
      depth++;
      i++;
    } else if (lineText[i] === "}" && i + 1 < lineText.length && lineText[i + 1] === "}") {
      depth--;
      i++;
    }
  }
  return depth > 0;
}

const lspToVscodeKind: Record<number, VCompletionItemKind> = {
  [LSPCompletionItemKind.Text]: VCompletionItemKind.Text,
  [LSPCompletionItemKind.Method]: VCompletionItemKind.Method,
  [LSPCompletionItemKind.Function]: VCompletionItemKind.Function,
  [LSPCompletionItemKind.Constructor]: VCompletionItemKind.Constructor,
  [LSPCompletionItemKind.Field]: VCompletionItemKind.Field,
  [LSPCompletionItemKind.Variable]: VCompletionItemKind.Variable,
  [LSPCompletionItemKind.Class]: VCompletionItemKind.Class,
  [LSPCompletionItemKind.Interface]: VCompletionItemKind.Interface,
  [LSPCompletionItemKind.Module]: VCompletionItemKind.Module,
  [LSPCompletionItemKind.Property]: VCompletionItemKind.Property,
  [LSPCompletionItemKind.Unit]: VCompletionItemKind.Unit,
  [LSPCompletionItemKind.Value]: VCompletionItemKind.Value,
  [LSPCompletionItemKind.Enum]: VCompletionItemKind.Enum,
  [LSPCompletionItemKind.Keyword]: VCompletionItemKind.Keyword,
  [LSPCompletionItemKind.Snippet]: VCompletionItemKind.Snippet,
  [LSPCompletionItemKind.Color]: VCompletionItemKind.Color,
  [LSPCompletionItemKind.File]: VCompletionItemKind.File,
  [LSPCompletionItemKind.Reference]: VCompletionItemKind.Reference,
  [LSPCompletionItemKind.Folder]: VCompletionItemKind.Folder,
  [LSPCompletionItemKind.EnumMember]: VCompletionItemKind.EnumMember,
  [LSPCompletionItemKind.Constant]: VCompletionItemKind.Constant,
  [LSPCompletionItemKind.Struct]: VCompletionItemKind.Struct,
  [LSPCompletionItemKind.Event]: VCompletionItemKind.Event,
  [LSPCompletionItemKind.Operator]: VCompletionItemKind.Operator,
  [LSPCompletionItemKind.TypeParameter]: VCompletionItemKind.TypeParameter,
};

export function activate(context: ExtensionContext): void {
  const config = workspace.getConfiguration("gohtml-lsp");
  let serverPath = config.get<string>("serverPath", "");

  if (!serverPath) {
    const platformMap: Record<string, string> = {
      "darwin-arm64": "gohtml-lsp-darwin-arm64",
      "darwin-x64": "gohtml-lsp-darwin-amd64",
      "linux-x64": "gohtml-lsp-linux-amd64",
      "linux-arm64": "gohtml-lsp-linux-arm64",
      "win32-x64": "gohtml-lsp-windows-amd64.exe",
    };

    const key = `${process.platform}-${process.arch}`;
    const binaryName = platformMap[key];

    if (!binaryName) {
      const msg = `Unsupported platform: ${key}. Supported: ${Object.keys(platformMap).join(", ")}`;
      window.showErrorMessage(msg);
      return;
    }

    const bundled = context.asAbsolutePath(`bin/${binaryName}`);
    if (existsSync(bundled)) {
      // Ensure the binary is executable on Linux/macOS (.vsix packaging can strip permissions)
      if (process.platform !== "win32") {
        try { chmodSync(bundled, 0o755); } catch { /* ignore */ }
      }
      serverPath = bundled;
    } else {
      // Fall back to PATH
      serverPath = process.platform === "win32" ? "gohtml-lsp.exe" : "gohtml-lsp";
    }
  }

  const outputChannel = window.createOutputChannel(
    "Go HTML Language Server",
    { log: true }
  );
  outputChannel.appendLine(`Starting gohtml-lsp from: ${serverPath}`);

  // Ensure the LSP child process can find `go` on Linux/macOS.
  // Snap, Homebrew, and manual installs place the binary in paths
  // that VS Code may not inherit.
  const extraPaths = [
    "/snap/bin",
    "/usr/local/go/bin",
    "/usr/local/bin",
    path.join(process.env.HOME || "", "go/bin"),
    path.join(process.env.HOME || "", ".local/bin"),
  ];
  const envPATH = process.env.PATH || "";
  const augmentedPATH = [...extraPaths, envPATH].join(path.delimiter);

  const run: Executable = {
    command: serverPath,
    transport: TransportKind.stdio,
    options: {
      env: { ...process.env, PATH: augmentedPATH },
    },
  };

  const serverOptions: ServerOptions = {
    run,
    debug: run,
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "gohtml" },
    ],
    synchronize: {
      fileEvents: [
        workspace.createFileSystemWatcher("**/*.gohtml"),
        workspace.createFileSystemWatcher("**/*.gotmpl"),
        workspace.createFileSystemWatcher("**/*.tmpl"),
        workspace.createFileSystemWatcher("**/*.tpl"),
        workspace.createFileSystemWatcher("**/*.go"),
      ],
    },
    outputChannel,
    traceOutputChannel: outputChannel,
  };

  client = new LanguageClient(
    "gohtml-lsp",
    "Go HTML Language Server",
    serverOptions,
    clientOptions
  );

  client.start().then(() => {
    outputChannel.appendLine("gohtml-lsp started successfully.");
  }).catch((err) => {
    const msg =
      `Failed to start Go HTML Language Server: ${err.message}. ` +
      `Binary path: ${serverPath}. ` +
      `Set gohtml-lsp.serverPath in settings if needed.`;
    outputChannel.appendLine(msg);
    window.showErrorMessage(msg);
  });

  // HTML language features for content outside {{ }} actions
  const htmlLS: HTMLLanguageService = getHTMLLanguageService();
  const selector = { scheme: "file", language: "gohtml" };

  const htmlCompletionProvider = languages.registerCompletionItemProvider(
    selector,
    {
      provideCompletionItems(
        doc: VTextDocument,
        position: VPosition,
        _token: CancellationToken,
        _ctx: CompletionContext
      ): CompletionList | undefined {
        const lineText = doc.lineAt(position.line).text;
        if (isInsideTemplateAction(lineText, position.character)) {
          return undefined;
        }

        const strippedText = stripTemplateActions(doc.getText());
        const lspDoc = LSPTextDocument.create(
          doc.uri.toString(),
          "html",
          doc.version,
          strippedText
        );
        const htmlDoc = htmlLS.parseHTMLDocument(lspDoc);
        const lspPos = { line: position.line, character: position.character };
        const result = htmlLS.doComplete(lspDoc, lspPos, htmlDoc);

        const items = result.items.map((item) => {
          const ci = new VCompletionItem(
            item.label,
            item.kind != null ? (lspToVscodeKind[item.kind] ?? VCompletionItemKind.Text) : VCompletionItemKind.Text
          );
          if (item.documentation) {
            ci.documentation = typeof item.documentation === "string"
              ? item.documentation
              : new MarkdownString(item.documentation.value);
          }
          if (item.textEdit && "newText" in item.textEdit) {
            if (item.insertTextFormat === InsertTextFormat.Snippet) {
              ci.insertText = new SnippetString(item.textEdit.newText);
            } else {
              ci.insertText = item.textEdit.newText;
            }
            const r = item.textEdit.range;
            ci.range = new VRange(
              new VPosition(r.start.line, r.start.character),
              new VPosition(r.end.line, r.end.character)
            );
          } else if (item.insertText) {
            if (item.insertTextFormat === InsertTextFormat.Snippet) {
              ci.insertText = new SnippetString(item.insertText);
            } else {
              ci.insertText = item.insertText;
            }
          }
          if (item.detail) {
            ci.detail = item.detail;
          }
          return ci;
        });

        return new CompletionList(items, result.isIncomplete);
      },
    },
    "<",
    "/",
    " "
  );

  const htmlHoverProvider = languages.registerHoverProvider(selector, {
    provideHover(
      doc: VTextDocument,
      position: VPosition,
      _token: CancellationToken
    ): VHover | undefined {
      const lineText = doc.lineAt(position.line).text;
      if (isInsideTemplateAction(lineText, position.character)) {
        return undefined;
      }

      const strippedText = stripTemplateActions(doc.getText());
      const lspDoc = LSPTextDocument.create(
        doc.uri.toString(),
        "html",
        doc.version,
        strippedText
      );
      const htmlDoc = htmlLS.parseHTMLDocument(lspDoc);
      const lspPos = { line: position.line, character: position.character };
      const result = htmlLS.doHover(lspDoc, lspPos, htmlDoc);

      if (!result) {
        return undefined;
      }

      const contents = result.contents;
      let md: MarkdownString;
      if (typeof contents === "string") {
        md = new MarkdownString(contents);
      } else if ("kind" in contents) {
        md = new MarkdownString(contents.value);
      } else if (Array.isArray(contents)) {
        md = new MarkdownString(
          contents.map((c) => (typeof c === "string" ? c : c.value)).join("\n\n")
        );
      } else {
        md = new MarkdownString(contents.value);
      }

      let range: VRange | undefined;
      if (result.range) {
        range = new VRange(
          new VPosition(result.range.start.line, result.range.start.character),
          new VPosition(result.range.end.line, result.range.end.character)
        );
      }

      return new VHover(md, range);
    },
  });

  // Static asset path completion inside src="", href="", action="", poster=""
  const pathAttrPattern = /\b(?:src|href|action|poster|data)\s*=\s*"([^"]*)/;

  const staticPathProvider = languages.registerCompletionItemProvider(
    selector,
    {
      provideCompletionItems(
        doc: VTextDocument,
        position: VPosition,
      ): VCompletionItem[] | undefined {
        const lineText = doc.lineAt(position.line).text;
        if (isInsideTemplateAction(lineText, position.character)) {
          return undefined;
        }

        // Check if cursor is inside a relevant attribute value
        const beforeCursor = lineText.substring(0, position.character);
        const match = pathAttrPattern.exec(beforeCursor);
        if (!match) {
          return undefined;
        }

        const attrValue = match[1]; // e.g. "/assets/css" or "/assets/"
        if (!attrValue.startsWith("/")) {
          return undefined;
        }

        const staticRoot = workspace.getConfiguration("gohtml-lsp").get<string>("staticRoot", "");
        if (!staticRoot) {
          return undefined;
        }

        const workspaceFolder = workspace.workspaceFolders?.[0]?.uri.fsPath;
        if (!workspaceFolder) {
          return undefined;
        }

        const absStaticRoot = path.isAbsolute(staticRoot)
          ? staticRoot
          : path.join(workspaceFolder, staticRoot);

        if (!existsSync(absStaticRoot)) {
          return undefined;
        }

        // Map the URL path to the filesystem path
        // attrValue is like "/assets/css/" — strip leading "/"
        const urlPath = attrValue.substring(1); // "assets/css/"
        const targetDir = path.join(absStaticRoot, urlPath);

        // If targetDir doesn't end with / and isn't a directory, go to parent
        let dirToList: string;
        let prefix: string;
        if (existsSync(targetDir) && statSync(targetDir).isDirectory()) {
          dirToList = targetDir;
          prefix = "";
        } else {
          dirToList = path.dirname(targetDir);
          prefix = path.basename(targetDir);
        }

        if (!existsSync(dirToList)) {
          return undefined;
        }

        try {
          const entries = readdirSync(dirToList, { withFileTypes: true });
          return entries
            .filter((e) => !e.name.startsWith("."))
            .filter((e) => !prefix || e.name.toLowerCase().startsWith(prefix.toLowerCase()))
            .map((entry) => {
              const isDir = entry.isDirectory();
              const ci = new VCompletionItem(
                entry.name,
                isDir ? VCompletionItemKind.Folder : VCompletionItemKind.File,
              );
              if (isDir) {
                ci.insertText = entry.name + "/";
                ci.command = {
                  command: "editor.action.triggerSuggest",
                  title: "Re-trigger completions",
                };
              }
              const ext = path.extname(entry.name).toLowerCase();
              const detailMap: Record<string, string> = {
                ".css": "CSS Stylesheet",
                ".js": "JavaScript",
                ".ts": "TypeScript",
                ".png": "PNG Image",
                ".jpg": "JPEG Image",
                ".jpeg": "JPEG Image",
                ".gif": "GIF Image",
                ".svg": "SVG Image",
                ".webp": "WebP Image",
                ".ico": "Icon",
                ".woff": "Web Font",
                ".woff2": "Web Font",
                ".ttf": "TrueType Font",
                ".mp4": "MP4 Video",
                ".webm": "WebM Video",
                ".pdf": "PDF Document",
              };
              ci.detail = isDir ? "Directory" : (detailMap[ext] || ext.substring(1).toUpperCase() + " File");
              return ci;
            });
        } catch {
          return undefined;
        }
      },
    },
    "/",
  );

  context.subscriptions.push(
    outputChannel,
    htmlCompletionProvider,
    htmlHoverProvider,
    staticPathProvider,
    {
      dispose: () => {
        if (client) {
          client.stop();
        }
      },
    }
  );
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
