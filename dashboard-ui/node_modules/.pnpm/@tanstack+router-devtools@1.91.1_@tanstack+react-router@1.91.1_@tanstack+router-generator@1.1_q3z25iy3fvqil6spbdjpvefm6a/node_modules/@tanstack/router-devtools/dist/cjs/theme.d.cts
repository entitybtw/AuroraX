import { default as React } from 'react';
export declare const defaultTheme: {
    readonly background: "#222222";
    readonly backgroundAlt: "#292929";
    readonly foreground: "white";
    readonly gray: "#444";
    readonly grayAlt: "#444";
    readonly inputBackgroundColor: "#fff";
    readonly inputTextColor: "#000";
    readonly success: "#80cb00";
    readonly danger: "#ff0085";
    readonly active: "#0099ff";
    readonly warning: "#ffb200";
};
export type Theme = typeof defaultTheme;
interface ProviderProps {
    theme: Theme;
    children?: React.ReactNode;
}
export declare function ThemeProvider({ theme, ...rest }: ProviderProps): import("react/jsx-runtime").JSX.Element;
export declare function useTheme(): {
    readonly background: "#222222";
    readonly backgroundAlt: "#292929";
    readonly foreground: "white";
    readonly gray: "#444";
    readonly grayAlt: "#444";
    readonly inputBackgroundColor: "#fff";
    readonly inputTextColor: "#000";
    readonly success: "#80cb00";
    readonly danger: "#ff0085";
    readonly active: "#0099ff";
    readonly warning: "#ffb200";
};
export {};
