declare module 'sql.js/dist/sql-wasm-browser.js' {
  const initSqlJs: (config?: {
    locateFile?: (file: string) => string;
    wasmBinary?: Uint8Array;
  }) => Promise<{
    Database: new (data?: Uint8Array) => {
      run: (sql: string, params?: Record<string, unknown>) => void;
      exec: (sql: string, params?: Record<string, unknown>) => Array<{ columns: string[]; values: unknown[][] }>;
      export: () => Uint8Array;
    };
  }>;

  export default initSqlJs;
}
