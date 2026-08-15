//#region src/core/compat.d.ts
/**
 * References
 * - https://compat-table.github.io/compat-table/es6/
 * - MDN
 */
declare enum Feature {
  AggregateError = 1,
  ArrowFunction = 2,
  ErrorPrototypeStack = 4,
  ObjectAssign = 8,
  BigIntTypedArray = 16,
  RegExp = 32,
  Temporal = 64
}
//#endregion
//#region src/core/constants.d.ts
declare const enum SerovalConstant {
  Null = 0,
  Undefined = 1,
  True = 2,
  False = 3,
  NegZero = 4,
  Inf = 5,
  NegInf = 6,
  Nan = 7
}
declare const enum SerovalNodeType {
  Number = 0,
  String = 1,
  Constant = 2,
  BigInt = 3,
  IndexedValue = 4,
  Date = 5,
  RegExp = 6,
  Set = 7,
  Map = 8,
  Array = 9,
  Object = 10,
  NullConstructor = 11,
  Promise = 12,
  Error = 13,
  AggregateError = 14,
  TypedArray = 15,
  BigIntTypedArray = 16,
  WKSymbol = 17,
  Reference = 18,
  ArrayBuffer = 19,
  DataView = 20,
  Boxed = 21,
  PromiseConstructor = 22,
  PromiseSuccess = 23,
  PromiseFailure = 24,
  Plugin = 25,
  SpecialReference = 26,
  IteratorFactory = 27,
  IteratorFactoryInstance = 28,
  AsyncIteratorFactory = 29,
  AsyncIteratorFactoryInstance = 30,
  StreamConstructor = 31,
  StreamNext = 32,
  StreamThrow = 33,
  StreamReturn = 34,
  Sequence = 35,
  Temporal = 36
}
declare const enum SerovalTemporalType {
  Instant = 0,
  Duration = 1,
  PlainDate = 2,
  PlainDateTime = 3,
  PlainMonthDay = 4,
  PlainTime = 5,
  PlainYearMonth = 6,
  ZonedDateTime = 7
}
declare const enum SerovalObjectFlags {
  None = 0,
  NonExtensible = 1,
  Sealed = 2,
  Frozen = 3
}
declare const enum Symbols {
  AsyncIterator = 0,
  HasInstance = 1,
  IsConcatSpreadable = 2,
  Iterator = 3,
  Match = 4,
  MatchAll = 5,
  Replace = 6,
  Search = 7,
  Species = 8,
  Split = 9,
  ToPrimitive = 10,
  ToStringTag = 11,
  Unscopables = 12
}
declare const enum ErrorConstructorTag {
  Error = 0,
  EvalError = 1,
  RangeError = 2,
  ReferenceError = 3,
  SyntaxError = 4,
  TypeError = 5,
  URIError = 6
}
//#endregion
//#region src/core/special-reference.d.ts
declare const enum SpecialReference {
  MapSentinel = 0,
  PromiseConstructor = 1,
  PromiseSuccess = 2,
  PromiseFailure = 3,
  StreamConstructor = 4,
  ArrayBufferConstructor = 5
}
//#endregion
//#region src/core/types.d.ts
interface SerovalBaseNode {
  t: SerovalNodeType;
  i: number | undefined;
  s: unknown;
  c: string | SerovalTemporalType | undefined;
  m: string | undefined;
  p: SerovalObjectRecordNode | undefined;
  e: SerovalMapRecordNode | undefined;
  a: (SerovalNode | 0)[] | undefined;
  f: SerovalNode | undefined;
  b: number | undefined;
  o: SerovalObjectFlags | undefined;
  l: number | undefined;
}
type SerovalObjectRecordKey = string | SerovalNode;
interface SerovalObjectRecordNode {
  k: SerovalObjectRecordKey[];
  v: SerovalNode[];
}
interface SerovalMapRecordNode {
  k: SerovalNode[];
  v: SerovalNode[];
}
interface SerovalNumberNode extends SerovalBaseNode {
  t: SerovalNodeType.Number;
  s: number;
}
interface SerovalStringNode extends SerovalBaseNode {
  t: SerovalNodeType.String;
  s: string;
}
interface SerovalConstantNode extends SerovalBaseNode {
  t: SerovalNodeType.Constant;
  s: SerovalConstant;
}
type SerovalPrimitiveNode = SerovalNumberNode | SerovalStringNode | SerovalConstantNode;
interface SerovalIndexedValueNode extends SerovalBaseNode {
  t: SerovalNodeType.IndexedValue;
  i: number;
}
interface SerovalBigIntNode extends SerovalBaseNode {
  t: SerovalNodeType.BigInt;
  s: string;
}
interface SerovalDateNode extends SerovalBaseNode {
  t: SerovalNodeType.Date;
  i: number;
  s: string;
}
interface SerovalRegExpNode extends SerovalBaseNode {
  t: SerovalNodeType.RegExp;
  i: number;
  c: string;
  m: string;
}
interface SerovalArrayBufferNode extends SerovalBaseNode {
  t: SerovalNodeType.ArrayBuffer;
  i: number;
  s: string;
  f: SerovalNodeWithID;
}
interface SerovalTypedArrayNode extends SerovalBaseNode {
  t: SerovalNodeType.TypedArray;
  i: number;
  c: string;
  f: SerovalNode;
  b: number;
  l: number;
}
interface SerovalBigIntTypedArrayNode extends SerovalBaseNode {
  t: SerovalNodeType.BigIntTypedArray;
  i: number;
  c: string;
  f: SerovalNode;
  b: number;
  l: number;
}
interface SerovalTemporalNode extends SerovalBaseNode {
  t: SerovalNodeType.Temporal;
  i: number;
  c: SerovalTemporalType;
  s: string;
}
type SerovalSemiPrimitiveNode = SerovalBigIntNode | SerovalDateNode | SerovalRegExpNode | SerovalTypedArrayNode | SerovalBigIntTypedArrayNode | SerovalTemporalNode;
interface SerovalSetNode extends SerovalBaseNode {
  t: SerovalNodeType.Set;
  i: number;
  a: SerovalNode[];
}
interface SerovalMapNode extends SerovalBaseNode {
  t: SerovalNodeType.Map;
  i: number;
  e: SerovalMapRecordNode;
  f: SerovalNodeWithID;
}
interface SerovalArrayNode extends SerovalBaseNode {
  t: SerovalNodeType.Array;
  a: (SerovalNode | 0)[];
  i: number;
  o: SerovalObjectFlags;
}
interface SerovalObjectNode extends SerovalBaseNode {
  t: SerovalNodeType.Object;
  p: SerovalObjectRecordNode;
  i: number;
  o: SerovalObjectFlags;
}
interface SerovalNullConstructorNode extends SerovalBaseNode {
  t: SerovalNodeType.NullConstructor;
  p: SerovalObjectRecordNode;
  i: number;
  o: SerovalObjectFlags;
}
interface SerovalPromiseNode extends SerovalBaseNode {
  t: SerovalNodeType.Promise;
  s: 0 | 1;
  f: SerovalNode;
  i: number;
}
interface SerovalErrorNode extends SerovalBaseNode {
  t: SerovalNodeType.Error;
  s: ErrorConstructorTag;
  m: string;
  p: SerovalObjectRecordNode | undefined;
  i: number;
}
interface SerovalAggregateErrorNode extends SerovalBaseNode {
  t: SerovalNodeType.AggregateError;
  i: number;
  m: string;
  p: SerovalObjectRecordNode | undefined;
}
interface SerovalWKSymbolNode extends SerovalBaseNode {
  t: SerovalNodeType.WKSymbol;
  i: number;
  s: Symbols;
}
interface SerovalReferenceNode extends SerovalBaseNode {
  t: SerovalNodeType.Reference;
  i: number;
  s: string;
}
interface SerovalDataViewNode extends SerovalBaseNode {
  t: SerovalNodeType.DataView;
  i: number;
  f: SerovalNode;
  b: number;
  l: number;
}
interface SerovalBoxedNode extends SerovalBaseNode {
  t: SerovalNodeType.Boxed;
  i: number;
  f: SerovalNode;
}
interface SerovalPromiseConstructorNode extends SerovalBaseNode {
  t: SerovalNodeType.PromiseConstructor;
  i: number;
  s: number;
  f: SerovalNodeWithID;
}
interface SerovalPromiseResolveNode extends SerovalBaseNode {
  t: SerovalNodeType.PromiseSuccess;
  i: number;
  a: [resolver: SerovalNodeWithID, resolved: SerovalNode];
}
interface SerovalPromiseRejectNode extends SerovalBaseNode {
  t: SerovalNodeType.PromiseFailure;
  i: number;
  a: [resolver: SerovalNodeWithID, resolved: SerovalNode];
}
interface SerovalPluginNode extends SerovalBaseNode {
  t: SerovalNodeType.Plugin;
  i: number;
  s: Record<string, SerovalNode>;
  c: string;
}
/**
 * Represents special values as placeholders
 */
interface SerovalSpecialReferenceNode extends SerovalBaseNode {
  t: SerovalNodeType.SpecialReference;
  i: number;
  s: SpecialReference;
}
interface SerovalIteratorFactoryNode extends SerovalBaseNode {
  t: SerovalNodeType.IteratorFactory;
  i: number;
  f: SerovalNodeWithID;
}
interface SerovalIteratorFactoryInstanceNode extends SerovalBaseNode {
  t: SerovalNodeType.IteratorFactoryInstance;
  a: [instance: SerovalNodeWithID, sequence: SerovalNodeWithID];
}
interface SerovalAsyncIteratorFactoryNode extends SerovalBaseNode {
  t: SerovalNodeType.AsyncIteratorFactory;
  i: number;
  a: [promise: SerovalNodeWithID, symbol: SerovalNodeWithID];
}
interface SerovalAsyncIteratorFactoryInstanceNode extends SerovalBaseNode {
  t: SerovalNodeType.AsyncIteratorFactoryInstance;
  a: [instance: SerovalNodeWithID, sequence: SerovalNodeWithID];
}
interface SerovalStreamConstructorNode extends SerovalBaseNode {
  t: SerovalNodeType.StreamConstructor;
  i: number;
  a: SerovalNode[];
  f: SerovalNodeWithID;
}
interface SerovalStreamNextNode extends SerovalBaseNode {
  t: SerovalNodeType.StreamNext;
  i: number;
  f: SerovalNode;
}
interface SerovalStreamThrowNode extends SerovalBaseNode {
  t: SerovalNodeType.StreamThrow;
  i: number;
  f: SerovalNode;
}
interface SerovalStreamReturnNode extends SerovalBaseNode {
  t: SerovalNodeType.StreamReturn;
  i: number;
  f: SerovalNode;
}
interface SerovalSequenceNode extends SerovalBaseNode {
  t: SerovalNodeType.Sequence;
  i: number;
  s: number;
  a: SerovalNode[];
  l: number;
}
type SerovalSyncNode = SerovalPrimitiveNode | SerovalIndexedValueNode | SerovalSemiPrimitiveNode | SerovalSetNode | SerovalMapNode | SerovalArrayNode | SerovalObjectNode | SerovalNullConstructorNode | SerovalPromiseNode | SerovalErrorNode | SerovalAggregateErrorNode | SerovalWKSymbolNode | SerovalReferenceNode | SerovalArrayBufferNode | SerovalDataViewNode | SerovalBoxedNode | SerovalPluginNode | SerovalSpecialReferenceNode | SerovalIteratorFactoryNode | SerovalIteratorFactoryInstanceNode | SerovalAsyncIteratorFactoryNode | SerovalAsyncIteratorFactoryInstanceNode | SerovalSequenceNode;
type SerovalAsyncNode = SerovalPromiseNode | SerovalPromiseConstructorNode | SerovalPromiseResolveNode | SerovalPromiseRejectNode | SerovalStreamConstructorNode | SerovalStreamNextNode | SerovalStreamThrowNode | SerovalStreamReturnNode;
type SerovalNode = SerovalSyncNode | SerovalAsyncNode;
type SerovalNodeWithID = Extract<SerovalNode, {
  i: number;
}>;
//#endregion
//#region src/core/context/deserializer.d.ts
interface BaseDeserializerContextOptions extends PluginAccessOptions {
  refs?: Map<number, unknown>;
  features?: number;
  disabledFeatures?: number;
  depthLimit?: number;
}
interface BaseDeserializerContext extends PluginAccessOptions {
  readonly mode: SerovalMode;
  /**
   * Mapping ids to values
   */
  refs: Map<number, unknown> & {
    types: Map<number, SerovalNodeType>;
  };
  features: number;
  depthLimit: number;
}
interface VanillaDeserializerContextOptions extends Omit<BaseDeserializerContextOptions, 'refs'> {
  markedRefs: number[] | Set<number>;
}
interface VanillaDeserializerState {
  marked: Set<number>;
}
interface VanillaDeserializerContext {
  mode: SerovalMode.Vanilla;
  base: BaseDeserializerContext;
  child: DeserializePluginContext | undefined;
  state: VanillaDeserializerState;
}
interface CrossDeserializerContext {
  mode: SerovalMode.Cross;
  base: BaseDeserializerContext;
  child: DeserializePluginContext | undefined;
}
type CrossDeserializerContextOptions = BaseDeserializerContextOptions;
type DeserializerContext = VanillaDeserializerContext | CrossDeserializerContext;
declare class DeserializePluginContext {
  private _p;
  private depth;
  constructor(_p: DeserializerContext, depth: number);
  deserialize<T>(node: SerovalNode): T;
}
//#endregion
//#region src/core/context/serializer.d.ts
declare const enum AssignmentType {
  Index = 0,
  Add = 1,
  Set = 2,
  Delete = 3,
  Define = 4
}
interface IndexAssignment {
  t: AssignmentType.Index;
  s: string;
  k: undefined;
  v: string;
}
interface SetAssignment {
  t: AssignmentType.Set;
  s: string;
  k: string;
  v: string;
}
interface AddAssignment {
  t: AssignmentType.Add;
  s: string;
  k: undefined;
  v: string;
}
interface DeleteAssignment {
  t: AssignmentType.Delete;
  s: string;
  k: string;
  v: undefined;
}
interface DefineAssignment {
  t: AssignmentType.Define;
  s: string;
  k: string;
  v: undefined;
}
type Assignment = IndexAssignment | AddAssignment | SetAssignment | DeleteAssignment | DefineAssignment;
interface FlaggedObject {
  type: SerovalObjectFlags;
  value: string;
}
interface BaseSerializerContextOptions extends PluginAccessOptions {
  features: number;
  markedRefs: number[] | Set<number>;
}
interface BaseSerializerContext extends PluginAccessOptions {
  readonly mode: SerovalMode;
  features: number;
  stack: number[];
  /**
   * Array of object mutations
   */
  flags: FlaggedObject[];
  /**
   * Array of assignments to be done (used for recursion)
   */
  assignments: Assignment[];
  /**
   * Refs that are...referenced
   */
  marked: Set<number>;
}
interface CrossContextOptions {
  scopeId?: string;
}
interface VanillaSerializerState {
  valid: Map<number, number>;
  vars: string[];
}
interface VanillaSerializerContext {
  mode: SerovalMode.Vanilla;
  base: BaseSerializerContext;
  state: VanillaSerializerState;
  child: SerializePluginContext | undefined;
}
type VanillaSerializerContextOptions = BaseSerializerContextOptions;
interface CrossSerializerContext {
  mode: SerovalMode.Cross;
  base: BaseSerializerContext;
  state: CrossContextOptions;
  child: SerializePluginContext | undefined;
}
interface CrossSerializerContextOptions extends BaseSerializerContextOptions, CrossContextOptions {}
type SerializerContext = VanillaSerializerContext | CrossSerializerContext;
declare class SerializePluginContext {
  private _p;
  constructor(_p: SerializerContext);
  serialize(node: SerovalNode): string;
}
//#endregion
//#region src/core/context/parser.d.ts
interface BaseParserContextOptions extends PluginAccessOptions {
  disabledFeatures?: number;
  refs?: Map<unknown, number>;
  depthLimit?: number;
}
interface BaseParserContext extends PluginAccessOptions {
  readonly mode: SerovalMode;
  marked: Set<number>;
  refs: Map<unknown, number>;
  features: number;
  depthLimit: number;
}
//#endregion
//#region src/core/context/sync-parser.d.ts
type SyncParserContextOptions = BaseParserContextOptions;
declare const enum ParserMode {
  Sync = 1,
  Stream = 2
}
interface SyncParserContext {
  type: ParserMode.Sync;
  base: BaseParserContext;
  child: SyncParsePluginContext | undefined;
}
declare class SyncParsePluginContext {
  private _p;
  private depth;
  constructor(_p: SyncParserContext, depth: number);
  parse<T>(current: T): SerovalNode;
}
interface StreamParserContextOptions extends SyncParserContextOptions {
  onParse: (node: SerovalNode, initial: boolean) => void;
  onError?: (error: unknown) => void;
  onDone?: () => void;
}
interface StreamParserContext {
  type: ParserMode.Stream;
  base: BaseParserContext;
  state: StreamParserState;
}
declare class StreamParsePluginContext {
  private _p;
  private depth;
  constructor(_p: StreamParserContext, depth: number);
  parse<T>(current: T): SerovalNode;
  parseWithError<T>(current: T): SerovalNode | undefined;
  isAlive(): boolean;
  pushPendingState(): void;
  popPendingState(): void;
  onParse(node: SerovalNode): void;
  onError(error: unknown): void;
  addCleanup(callback: () => void): void;
}
interface StreamParserState {
  alive: boolean;
  pending: number;
  initial: boolean;
  buffer: SerovalNode[];
  onParse: (node: SerovalNode, initial: boolean) => void;
  onError?: (error: unknown) => void;
  onDone?: () => void;
  cleanups: (() => void)[];
}
//#endregion
//#region src/core/plugin.d.ts
declare const enum SerovalMode {
  Vanilla = 1,
  Cross = 2
}
interface PluginData {
  id: number;
}
type PluginInfo = {
  [key: string]: SerovalNode;
};
interface Plugin<Value, Info extends PluginInfo> {
  /**
   * A unique string that helps idenfity the plugin
   */
  tag: string;
  /**
   * List of dependency plugins
   */
  extends?: Plugin<any, any>[];
  /**
   * Method to test if a value is an expected value of the plugin
   * @param value
   */
  test(value: unknown): boolean;
  /**
   * Parsing modes
   */
  parse: {
    sync?: (value: Value, ctx: SyncParsePluginContext, data: PluginData) => Info;
    async?: (value: Value, ctx: AsyncParsePluginContext, data: PluginData) => Promise<Info>;
    stream?: (value: Value, ctx: StreamParsePluginContext, data: PluginData) => Info;
  };
  /**
   * Convert the parsed node into a JS string
   */
  serialize(node: Info, ctx: SerializePluginContext, data: PluginData): string;
  /**
   * Convert the parsed node into its runtime equivalent.
   */
  deserialize(node: Info, ctx: DeserializePluginContext, data: PluginData): Value;
}
declare function createPlugin<Value, Info extends PluginInfo>(plugin: Plugin<Value, Info>): Plugin<Value, Info>;
interface PluginAccessOptions {
  plugins?: Plugin<any, any>[];
}
declare function resolvePlugins(plugins?: Plugin<any, any>[]): Plugin<any, any>[] | undefined;
//#endregion
//#region src/core/context/async-parser.d.ts
type AsyncParserContextOptions = BaseParserContextOptions;
interface AsyncParserContext {
  base: BaseParserContext;
  child: AsyncParsePluginContext | undefined;
}
declare class AsyncParsePluginContext {
  private _p;
  private depth;
  constructor(_p: AsyncParserContext, depth: number);
  parse<T>(current: T): Promise<SerovalNode>;
}
//#endregion
//#region src/core/cross/index.d.ts
interface CrossSerializeOptions extends SyncParserContextOptions, CrossContextOptions {}
declare function crossSerialize<T>(source: T, options?: CrossSerializeOptions): string;
interface CrossSerializeAsyncOptions extends AsyncParserContextOptions, CrossContextOptions {}
declare function crossSerializeAsync<T>(source: T, options?: CrossSerializeAsyncOptions): Promise<string>;
type ToCrossJSONOptions = SyncParserContextOptions;
declare function toCrossJSON<T>(source: T, options?: ToCrossJSONOptions): SerovalNode;
type ToCrossJSONAsyncOptions = AsyncParserContextOptions;
declare function toCrossJSONAsync<T>(source: T, options?: ToCrossJSONAsyncOptions): Promise<SerovalNode>;
interface CrossSerializeStreamOptions extends Omit<StreamParserContextOptions, 'onParse'>, CrossContextOptions {
  onSerialize: (data: string, initial: boolean) => void;
}
declare function crossSerializeStream<T>(source: T, options: CrossSerializeStreamOptions): () => void;
type ToCrossJSONStreamOptions = StreamParserContextOptions;
declare function toCrossJSONStream<T>(source: T, options: ToCrossJSONStreamOptions): () => void;
type FromCrossJSONOptions = CrossDeserializerContextOptions;
declare function fromCrossJSON<T>(source: SerovalNode, options: FromCrossJSONOptions): T;
//#endregion
//#region src/core/errors.d.ts
declare class SerovalError extends Error {
  cause: unknown;
  constructor(type: string, cause: unknown);
}
declare class SerovalParserError extends SerovalError {
  constructor(cause: unknown);
}
declare class SerovalSerializationError extends SerovalError {
  constructor(cause: unknown);
}
declare class SerovalDeserializationError extends SerovalError {
  constructor(cause: unknown);
}
declare class SerovalUnsupportedTypeError extends Error {
  value: unknown;
  constructor(value: unknown);
}
declare class SerovalUnsupportedNodeError extends Error {
  constructor(node: SerovalNode);
}
declare class SerovalMissingPluginError extends Error {
  constructor(tag: string);
}
declare class SerovalMissingInstanceError extends Error {
  constructor(tag: string);
}
declare class SerovalMissingReferenceError extends Error {
  value: unknown;
  constructor(value: unknown);
}
declare class SerovalMissingReferenceForIdError extends Error {
  constructor(id: string);
}
declare class SerovalUnknownTypedArrayError extends Error {
  constructor(name: string);
}
declare class SerovalMalformedNodeError extends Error {
  constructor(node: SerovalNode);
}
declare class SerovalConflictedNodeIdError extends Error {
  constructor(node: SerovalNode);
}
declare class SerovalDepthLimitError extends Error {
  constructor(limit: number);
}
//#endregion
//#region src/core/keys.d.ts
declare function getCrossReferenceHeader(id?: string): string;
//#endregion
//#region src/core/opaque-reference.d.ts
/**
 * An opaque reference allows hiding values from the serializer.
 */
declare class OpaqueReference<V, R = undefined> {
  readonly value: V;
  readonly replacement?: R | undefined;
  constructor(value: V, replacement?: R | undefined);
}
//#endregion
//#region src/core/reference.d.ts
declare function createReference<T>(id: string, value: T): T;
//#endregion
//#region src/core/Serializer.d.ts
interface SerializerOptions extends PluginAccessOptions {
  globalIdentifier: string;
  scopeId?: string;
  disabledFeatures?: number;
  onData: (result: string) => void;
  onError: (error: unknown) => void;
  onDone?: () => void;
}
declare class Serializer {
  private options;
  private alive;
  private flushed;
  private done;
  private pending;
  private cleanups;
  private refs;
  private plugins?;
  constructor(options: SerializerOptions);
  keys: Set<string>;
  write(key: string, value: unknown): void;
  ids: number;
  private getNextID;
  push(value: unknown): string;
  flush(): void;
  close(): void;
}
//#endregion
//#region src/core/stream.d.ts
interface StreamListener<T> {
  next(value: T): void;
  throw(value: unknown): void;
  return(value: T): void;
}
interface Stream<T> {
  __SEROVAL_STREAM__: true;
  on(listener: StreamListener<T>): () => void;
  next(value: T): void;
  throw(value: unknown): void;
  return(value: T): void;
}
declare function createStream<T>(): Stream<T>;
//#endregion
//#region src/core/tree/index.d.ts
type SyncParserContextOptions$1 = Omit<BaseParserContextOptions, 'refs'>;
type AsyncParserContextOptions$1 = Omit<BaseParserContextOptions, 'refs'>;
declare function serialize<T>(source: T, options?: SyncParserContextOptions$1): string;
declare function serializeAsync<T>(source: T, options?: AsyncParserContextOptions$1): Promise<string>;
declare function deserialize<T>(source: string): T;
interface SerovalJSON {
  t: SerovalNode;
  f: number;
  m: number[];
}
interface FromJSONOptions extends PluginAccessOptions {
  disabledFeatures?: number;
}
declare function toJSON<T>(source: T, options?: SyncParserContextOptions$1): SerovalJSON;
declare function toJSONAsync<T>(source: T, options?: AsyncParserContextOptions$1): Promise<SerovalJSON>;
declare function compileJSON(source: SerovalJSON, options?: PluginAccessOptions): string;
declare function fromJSON<T>(source: SerovalJSON, options?: FromJSONOptions): T;
//#endregion
export { type AsyncParsePluginContext, AsyncParserContextOptions, type BaseDeserializerContextOptions, type BaseParserContextOptions, type BaseSerializerContextOptions, type CrossContextOptions, type CrossDeserializerContextOptions, CrossSerializeAsyncOptions, CrossSerializeOptions, CrossSerializeStreamOptions, type CrossSerializerContextOptions, type DeserializePluginContext, Feature, FromCrossJSONOptions, FromJSONOptions, OpaqueReference, Plugin, PluginAccessOptions, PluginData, PluginInfo, type SerializePluginContext, Serializer, SerovalConflictedNodeIdError, SerovalDepthLimitError, SerovalDeserializationError, SerovalError, SerovalJSON, SerovalMalformedNodeError, SerovalMissingInstanceError, SerovalMissingPluginError, SerovalMissingReferenceError, SerovalMissingReferenceForIdError, SerovalMode, type SerovalNode, SerovalParserError, SerovalSerializationError, SerovalUnknownTypedArrayError, SerovalUnsupportedNodeError, SerovalUnsupportedTypeError, type Stream, type StreamListener, type StreamParsePluginContext, type StreamParserContextOptions, type SyncParsePluginContext, SyncParserContextOptions, ToCrossJSONAsyncOptions, ToCrossJSONOptions, ToCrossJSONStreamOptions, type VanillaDeserializerContextOptions, type VanillaSerializerContextOptions, compileJSON, createPlugin, createReference, createStream, crossSerialize, crossSerializeAsync, crossSerializeStream, deserialize, fromCrossJSON, fromJSON, getCrossReferenceHeader, resolvePlugins, serialize, serializeAsync, toCrossJSON, toCrossJSONAsync, toCrossJSONStream, toJSON, toJSONAsync };