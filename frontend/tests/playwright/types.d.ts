declare module "oauth2-mock-server";

declare global {
  interface Window {
    /** Set by transcode-scrub.spec.ts init script before enableTranscodeUi patches config. */
    __realMediaAvailable?: boolean;
    globalVars?: {
      mediaAvailable?: boolean;
    };
  }
}

export {};
