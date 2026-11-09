import { fetchJSON, fetchURL } from "./utils";

type TOTPSetup = { transaction: string; uri: string };
type PasskeySetup = {
  transaction: string;
  publicKey: PublicKeyCredentialCreationOptions;
};

export type MFAStatus = { totp: boolean; passkeys: number };

export function getStatus() {
  return fetchJSON<MFAStatus>("/api/mfa", {});
}

export async function beginTOTP(currentPassword: string) {
  const res = await fetchURL("/api/mfa/totp", {
    method: "POST",
    body: JSON.stringify({ current_password: currentPassword }),
  });
  return res.json() as Promise<TOTPSetup>;
}

export function confirmTOTP(transaction: string, code: string) {
  return fetchURL("/api/mfa/totp/confirm", {
    method: "POST",
    body: JSON.stringify({ transaction, code }),
  });
}

export function removeTOTP(currentPassword: string) {
  return fetchURL("/api/mfa/totp", {
    method: "DELETE",
    body: JSON.stringify({ current_password: currentPassword }),
  });
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

function creationOptions(
  options: PublicKeyCredentialCreationOptions
): PublicKeyCredentialCreationOptions {
  return {
    ...options,
    challenge: decodeBase64(options.challenge as unknown as string),
    user: {
      ...options.user,
      id: decodeBase64(options.user.id as unknown as string),
    },
    excludeCredentials: options.excludeCredentials?.map((credential) => ({
      ...credential,
      id: decodeBase64(credential.id as unknown as string),
    })),
  };
}

function credentialPayload(
  credential: PublicKeyCredential
): Record<string, unknown> {
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: encodeBase64(credential.rawId),
    type: credential.type,
    response: {
      attestationObject: encodeBase64(response.attestationObject),
      clientDataJSON: encodeBase64(response.clientDataJSON),
    },
  };
}

export async function enrollPasskey(currentPassword: string) {
  const begin = await fetchURL("/api/mfa/passkey", {
    method: "POST",
    body: JSON.stringify({ current_password: currentPassword }),
  });
  const setup = (await begin.json()) as PasskeySetup;
  const credential = await navigator.credentials.create({
    publicKey: creationOptions(setup.publicKey),
  });
  if (!(credential instanceof PublicKeyCredential))
    throw new Error("passkey_failed");
  return fetchURL("/api/mfa/passkey/confirm", {
    method: "POST",
    body: JSON.stringify({
      transaction: setup.transaction,
      ...credentialPayload(credential),
    }),
  });
}
