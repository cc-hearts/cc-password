export type GetPromiseReturns<T extends (...args: any[]) => any> =
  ReturnType<T> extends Promise<infer r> ? r : never
