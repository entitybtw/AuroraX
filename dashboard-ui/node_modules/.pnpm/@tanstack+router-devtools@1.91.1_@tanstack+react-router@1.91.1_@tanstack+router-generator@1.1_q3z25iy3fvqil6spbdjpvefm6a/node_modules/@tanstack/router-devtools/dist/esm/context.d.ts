import { default as React } from 'react';
export declare const ShadowDomTargetContext: React.Context<ShadowRoot | undefined>;
export declare const DevtoolsOnCloseContext: React.Context<{
    onCloseClick: (e: React.MouseEvent<HTMLButtonElement>) => void;
} | undefined>;
export declare const useDevtoolsOnClose: () => {
    onCloseClick: (e: React.MouseEvent<HTMLButtonElement>) => void;
};
