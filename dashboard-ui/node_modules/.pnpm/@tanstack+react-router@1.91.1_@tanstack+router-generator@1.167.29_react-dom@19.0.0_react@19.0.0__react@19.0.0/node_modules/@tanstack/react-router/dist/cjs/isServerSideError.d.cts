export declare function isServerSideError(error: unknown): error is {
    __isServerError: true;
    data: Record<string, any>;
};
export declare function defaultDeserializeError(serializedData: Record<string, any>): any;
