import type { ErrorData } from "hls.js";
import type { HttpError } from "@/api/utils.ts";

/** What the player code catches or gets a promise rejected with */
export type PlayerError = Error | HttpError | ErrorData;

export interface ErrorFields {
  name?: string;
  message?: string;
  /** HTTP status of an `HttpError` */
  status?: number;
  /** hls.js `ErrorData.type` */
  type?: string;
  /** hls.js `ErrorData.details` */
  details?: string;
}

function onlyString(value?: string): string | undefined {
  return typeof value === "string" ? value : undefined;
}

export function errorFields(err?: PlayerError | null): ErrorFields {
  if (typeof err !== "object" || err === null) {
    return {};
  }
  const status = "status" in err ? err.status : undefined;
  return {
    name: onlyString("name" in err ? err.name : undefined),
    message: onlyString("message" in err ? err.message : undefined),
    status: typeof status === "number" ? status : undefined,
    type: onlyString("type" in err ? err.type : undefined),
    details: onlyString("details" in err ? err.details : undefined),
  };
}

export function errorName(err?: PlayerError | null): string | undefined {
  return errorFields(err).name;
}

export function errorMessage(err?: PlayerError | null): string | undefined {
  return errorFields(err).message;
}

export function httpStatusOf(err?: PlayerError | null): number | undefined {
  return errorFields(err).status;
}

/** The attach was cancelled through its `AbortSignal` */
export class AttachAbortError extends Error {
  override name = "AbortError";
  constructor() {
    super("HLS attach aborted");
  }
}

/** The attach did not become ready in time */
export class AttachTimeoutError extends Error {
  override name = "AttachTimeoutError";
  constructor() {
    super("HLS attach timed out");
  }
}

/** The browser native HLS playback failed */
export class NativeHlsError extends Error {
  override name = "NativeHlsError";
  /** `MediaError.code` of the failed element */
  readonly code: number | undefined;
  constructor(message: string, code: number | undefined) {
    super(message);
    this.code = code;
  }
}

/** The attach was abandoned because a replacement transcode session is being started */
export class TranscodeRecoverError extends Error {
  override name = "TranscodeRecoverError";
  constructor() {
    super("transcode session recovery");
  }
}
