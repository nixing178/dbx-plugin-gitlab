export {};

declare global {
  interface DbxPluginContext {
    connectionId?: string;
    connectionName?: string;
    databaseType?: string;
    [key: string]: unknown;
  }

  interface DbxPluginTheme {
    appearance: "light" | "dark";
    tokens?: Record<string, string>;
  }

  interface DbxPlugin {
    ready: Promise<void>;
    context: DbxPluginContext;
    locale: string;
    theme: DbxPluginTheme;
    capabilities: Record<string, boolean>;
    invoke<T = unknown>(
      method: string,
      params?: Record<string, unknown>,
      options?: { timeoutMs?: number },
    ): Promise<T>;
    onEvent?(fn: (event: { method: string; params: unknown }) => void): () => void;
    copy?(text: string): Promise<void> | void;
  }

  interface Window {
    dbxPlugin?: DbxPlugin;
  }
}
