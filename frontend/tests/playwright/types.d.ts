declare module "oauth2-mock-server";

declare global {
  interface Window {
    globalVars?: {
      ffmpegAvailable?: boolean;
    };
  }
}

export {};
