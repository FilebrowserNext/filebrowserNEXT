<template>
  <div id="login">
    <form novalidate @submit="submit">
      <img :src="logoURL" alt="File Browser Next" />
      <h1>{{ name }}</h1>
      <p v-if="reason != null" class="logout-message">
        {{ t(`login.logout_reasons.${reason}`) }}
      </p>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <template v-if="mfaTransaction === ''">
        <input
          autofocus
          class="input input--block"
          type="text"
          autocapitalize="off"
          v-model="username"
          :placeholder="t('login.username')"
        />
        <input
          class="input input--block"
          type="password"
          v-model="password"
          :placeholder="t('login.password')"
        />
        <input
          class="input input--block"
          v-if="createMode"
          type="password"
          v-model="passwordConfirm"
          :placeholder="t('login.passwordConfirm')"
        />

        <altcha-widget
          v-if="altcha"
          ref="captchaWidget"
          class="captcha"
          auto="onfocus"
          type="checkbox"
          @verified="onCaptchaVerified"
          @expired="onCaptchaExpired"
        ></altcha-widget>
        <input
          class="button button--block"
          type="submit"
          :value="createMode ? t('login.signup') : t('login.submit')"
        />
      </template>

      <template v-else>
        <input
          v-if="mfaMethods.includes('totp')"
          class="input input--block"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          v-model="totpCode"
          :placeholder="t('login.totpCode')"
        />
        <input
          v-if="mfaMethods.includes('totp')"
          class="button button--block"
          type="button"
          :value="t('login.verifyTotp')"
          @click="submitTOTP"
        />
        <input
          v-if="mfaMethods.includes('passkey')"
          class="button button--block"
          type="button"
          :value="t('login.usePasskey')"
          @click="submitPasskey"
        />
      </template>

      <p @click="toggleMode" v-if="signup" class="toggle-link">
        {{ createMode ? t("login.loginInstead") : t("login.createAnAccount") }}
      </p>
    </form>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import * as auth from "@/utils/auth";
import { altcha, name, logoURL, signup } from "@/utils/constants";
import type { AltchaWidgetElement } from "altcha";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// Define refs
const createMode = ref<boolean>(false);
const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");
const passwordConfirm = ref<string>("");
const totpCode = ref<string>("");
const mfaTransaction = ref<string>("");
const mfaMethods = ref<string[]>([]);
const captchaWidget = ref<AltchaWidgetElement | null>(null);
const captchaPayload = ref<string>("");

const route = useRoute();
const router = useRouter();
const { t, locale } = useI18n({});
// Define functions
const toggleMode = () => (createMode.value = !createMode.value);

const $showError = inject<IToastError>("$showError")!;

const reason = route.query["logout-reason"] ?? null;

const submit = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();

  const redirect = (route.query.redirect || "/files/") as string;

  if (createMode.value) {
    if (password.value !== passwordConfirm.value) {
      error.value = t("login.passwordsDontMatch");
      return;
    }
  }

  try {
    if (createMode.value) {
      const signupCaptcha = await nextCaptcha();
      if (signupCaptcha === null) return;
      await auth.signup(username.value.trim(), password.value, signupCaptcha);
    }

    const loginCaptcha = await nextCaptcha();
    if (loginCaptcha === null) return;

    const challenge = await auth.login(
      username.value.trim(),
      password.value,
      loginCaptcha
    );
    if (challenge === null) {
      router.push({ path: redirect });
    } else {
      mfaTransaction.value = challenge.transaction;
      mfaMethods.value = challenge.methods;
    }
  } catch (e: any) {
    // console.error(e);
    if (e instanceof StatusError) {
      if (e.status === 409) {
        error.value = t("login.usernameTaken");
      } else if (e.status === 403) {
        error.value = t("login.wrongCredentials");
      } else if (e.status === 400) {
        const match = e.message.match(/minimum length is (\d+)/);
        if (match) {
          error.value = t("login.passwordTooShort", { min: match[1] });
        } else {
          error.value = e.message;
        }
      } else if (e.status === 429) {
        error.value = t("errors.tooManyRequests");
      } else {
        $showError(e);
      }
    }
  }
};

const submitTOTP = async () => {
  try {
    await auth.completeTOTPLogin(mfaTransaction.value, totpCode.value);
    await router.push({ path: (route.query.redirect || "/files/") as string });
  } catch (e) {
    error.value =
      e instanceof StatusError && e.status === 429
        ? t("errors.tooManyRequests")
        : t("login.wrongCredentials");
  }
};

const submitPasskey = async () => {
  try {
    await auth.completePasskeyLogin(mfaTransaction.value);
    await router.push({ path: (route.query.redirect || "/files/") as string });
  } catch (e) {
    error.value =
      e instanceof Error && e.message === "passkey_failed"
        ? t("errors.passkeyFailed")
        : t("login.wrongCredentials");
  }
};

const onCaptchaVerified = (event: Event) => {
  captchaPayload.value =
    (event as CustomEvent<{ payload: string }>).detail?.payload ?? "";
};

const onCaptchaExpired = () => {
  captchaPayload.value = "";
};

/**
 * Returns a fresh captcha solution, solving one on demand when the widget has
 * not been verified yet. Solutions are single-use, so the widget is reset for
 * the next request. Returns "" when no captcha is configured and null (with an
 * error displayed) when verification could not be completed.
 */
const nextCaptcha = async (): Promise<string | null> => {
  if (!altcha) return "";
  const widget = captchaWidget.value;
  if (widget === null) return null;

  if (captchaPayload.value === "") {
    const result = await widget.verify().catch((e: unknown) => {
      console.error("captcha verification failed", e);
      return null;
    });
    captchaPayload.value = result?.payload ?? "";
  }

  const payload = captchaPayload.value;
  captchaPayload.value = "";
  widget.reset();

  if (payload === "") {
    error.value = t("login.captcha.verificationRequired");
    return null;
  }
  return payload;
};

const setupCaptcha = async () => {
  const captcha = await import("@/utils/captcha");
  captcha.registerCaptcha();

  const language = `fb-${locale.value}`;
  captcha.setCaptchaStrings(language, {
    error: t("login.captcha.error"),
    expired: t("login.captcha.expired"),
    label: t("login.captcha.label"),
    loading: t("login.captcha.loading"),
    reload: t("login.captcha.reload"),
    verify: t("login.captcha.verify"),
    verificationRequired: t("login.captcha.verificationRequired"),
    verified: t("login.captcha.verified"),
    verifying: t("login.captcha.verifying"),
    waitAlert: t("login.captcha.waitAlert"),
  });

  await customElements.whenDefined("altcha-widget");
  await captchaWidget.value?.configure({
    challenge: captcha.challengeURL,
    hideFooter: true,
    hideLogo: true,
    language,
  });
};

// Run hooks
onMounted(() => {
  if (!altcha) return;
  setupCaptcha().catch((e: unknown) => {
    console.error("captcha setup failed", e);
    error.value = t("login.captcha.error");
  });
});
</script>
