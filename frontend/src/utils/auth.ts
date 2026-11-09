import { useAuthStore } from "@/stores/auth";
import router from "@/router";
import type { JwtPayload } from "jwt-decode";
import { jwtDecode } from "jwt-decode";
import { authMethod, baseURL, noAuth, logoutPage } from "./constants";
import { StatusError } from "@/api/utils";
import { setSafeTimeout } from "@/api/utils";

export function parseToken(token: string) {
  // falsy or malformed jwt will throw InvalidTokenError
  const data = jwtDecode<JwtPayload & { user: IUser }>(token);

  document.cookie = `auth=${token}; Path=/; SameSite=Strict;`;

  localStorage.setItem("jwt", token);

  const authStore = useAuthStore();
  authStore.jwt = token;
  authStore.setUser(data.user);

  // proxy auth with custom logout subject to unknown external timeout
  if (logoutPage !== "/login" && authMethod === "proxy") {
    console.warn("idle timeout disabled with proxy auth and custom logout");
    return;
  }

  if (authStore.logoutTimer) {
    clearTimeout(authStore.logoutTimer);
  }

  const expiresAt = new Date(data.exp! * 1000);
  const timeout = expiresAt.getTime() - Date.now();
  authStore.setLogoutTimer(
    setSafeTimeout(() => {
      logout("inactivity");
    }, timeout)
  );
}

export async function validateLogin() {
  try {
    if (localStorage.getItem("jwt")) {
      await renew(<string>localStorage.getItem("jwt"));
    }
  } catch (error) {
    console.warn("Invalid JWT token in storage");
    throw error;
  }
}

export interface MFAChallenge {
  transaction: string;
  methods: string[];
}

type PublicKeyEnvelope = {
  transaction: string;
  publicKey: PublicKeyCredentialRequestOptions;
};

export async function login(
  username: string,
  password: string,
  recaptcha: string
): Promise<MFAChallenge | null> {
  const data = { username, password, recaptcha };

  const res = await fetch(`${baseURL}/api/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  const body = await res.text();

  if (res.status === 200) {
    parseToken(body);
    return null;
  } else if (res.status === 202) {
    return JSON.parse(body) as MFAChallenge;
  } else {
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export async function completeTOTPLogin(transaction: string, code: string) {
  const res = await fetch(`${baseURL}/api/login/mfa/totp`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ transaction, code }),
  });
  const body = await res.text();
  if (res.status === 200) return parseToken(body);
  throw new StatusError(body || `${res.status} ${res.statusText}`, res.status);
}

function decodeBase64(value: string): ArrayBuffer {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(
    normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=")
  );
  return Uint8Array.from(binary, (char) => char.charCodeAt(0)).buffer;
}

function encodeBase64(value: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(value)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

function requestOptions(
  options: PublicKeyCredentialRequestOptions
): PublicKeyCredentialRequestOptions {
  return {
    ...options,
    challenge: decodeBase64(options.challenge as unknown as string),
    allowCredentials: options.allowCredentials?.map((credential) => ({
      ...credential,
      id: decodeBase64(credential.id as unknown as string),
    })),
  };
}

function passkeyPayload(
  credential: PublicKeyCredential
): Record<string, unknown> {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: encodeBase64(credential.rawId),
    type: credential.type,
    response: {
      authenticatorData: encodeBase64(response.authenticatorData),
      clientDataJSON: encodeBase64(response.clientDataJSON),
      signature: encodeBase64(response.signature),
      userHandle: response.userHandle
        ? encodeBase64(response.userHandle)
        : null,
    },
  };
}

export async function completePasskeyLogin(transaction: string) {
  const begin = await fetch(`${baseURL}/api/login/mfa/passkey/begin`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ transaction }),
  });
  const envelope = (await begin.json()) as PublicKeyEnvelope;
  if (!begin.ok) throw new StatusError("passkey_failed", begin.status);
  const credential = await navigator.credentials.get({
    publicKey: requestOptions(envelope.publicKey),
  });
  if (!(credential instanceof PublicKeyCredential))
    throw new Error("passkey_failed");
  const finish = await fetch(`${baseURL}/api/login/mfa/passkey`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      transaction: envelope.transaction,
      ...passkeyPayload(credential),
    }),
  });
  const body = await finish.text();
  if (finish.status === 200) return parseToken(body);
  throw new StatusError(
    body || `${finish.status} ${finish.statusText}`,
    finish.status
  );
}

export async function renew(jwt: string) {
  const res = await fetch(`${baseURL}/api/renew`, {
    method: "POST",
    headers: {
      "X-Auth": jwt,
    },
  });

  const body = await res.text();

  if (res.status === 200) {
    parseToken(body);
  } else {
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export async function signup(username: string, password: string) {
  const data = { username, password };

  const res = await fetch(`${baseURL}/api/signup`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (res.status !== 200) {
    const body = await res.text();
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export async function logout(reason?: string) {
  const token = localStorage.getItem("jwt");
  if (token) {
    try {
      await fetch(`${baseURL}/api/logout`, {
        method: "POST",
        headers: {
          "X-Auth": token,
        },
      });
    } catch {
      // ignore network errors on logout
    }
  }

  document.cookie = "auth=; Max-Age=0; Path=/; SameSite=Strict;";

  const authStore = useAuthStore();
  authStore.clearUser();

  localStorage.setItem("jwt", "");
  if (noAuth) {
    window.location.reload();
  } else if (logoutPage !== "/login") {
    document.location.href = `${logoutPage}`;
  } else {
    if (typeof reason === "string" && reason.trim() !== "") {
      router.push({
        path: "/login",
        query: { "logout-reason": reason },
      });
    } else {
      router.push({
        path: "/login",
      });
    }
  }
}
