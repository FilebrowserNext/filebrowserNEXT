// Self-hosted ALTCHA proof-of-work captcha.
//
// The "external" build of the widget ships no inline workers, so the SHA
// worker is bundled by Vite and served from our own origin. This keeps the
// default Content-Security-Policy (`default-src 'self'`) intact: no blob:
// workers and no third-party requests.
import "altcha/external";
import "altcha/altcha.css";
import ShaWorker from "altcha/workers/sha?worker";

import { baseURL } from "@/utils/constants";

const ALGORITHM = "SHA-256";

export const challengeURL = `${baseURL}/api/captcha/challenge`;

export interface CaptchaStrings {
  error: string;
  expired: string;
  label: string;
  loading: string;
  reload: string;
  verify: string;
  verificationRequired: string;
  verified: string;
  verifying: string;
  waitAlert: string;
}

let registered = false;

/** Registers the local SHA worker once per page. */
export function registerCaptcha(): void {
  if (registered) return;
  globalThis.$altcha.algorithms.set(ALGORITHM, () => new ShaWorker());
  registered = true;
}

/**
 * Publishes the application's translated strings to the widget under a
 * dedicated language tag, so the widget follows the interface language.
 */
export function setCaptchaStrings(language: string, strings: CaptchaStrings) {
  globalThis.$altcha.i18n.set(language, {
    ...strings,
    ariaLinkLabel: "",
    footer: "",
  });
}
