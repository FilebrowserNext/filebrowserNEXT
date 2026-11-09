<template>
  <div class="row">
    <div class="column">
      <form class="card" @submit="updateSettings">
        <div class="card-title">
          <h2>{{ t("settings.profileSettings") }}</h2>
        </div>

        <div class="card-content">
          <p>
            <input type="checkbox" name="hideDotfiles" v-model="hideDotfiles" />
            {{ t("settings.hideDotfiles") }}
          </p>
          <p>
            <input type="checkbox" name="singleClick" v-model="singleClick" />
            {{ t("settings.singleClick") }}
          </p>
          <p>
            <input
              type="checkbox"
              name="redirectAfterCopyMove"
              v-model="redirectAfterCopyMove"
            />
            {{ t("settings.redirectAfterCopyMove") }}
          </p>
          <p>
            <input type="checkbox" name="dateFormat" v-model="dateFormat" />
            {{ t("settings.setDateFormat") }}
          </p>
          <h3>{{ t("settings.language") }}</h3>
          <languages
            class="input input--block"
            v-model:locale="locale"
          ></languages>

          <h3>{{ t("settings.aceEditorTheme") }}</h3>
          <AceEditorTheme
            class="input input--block"
            v-model:aceEditorTheme="aceEditorTheme"
            id="aceTheme"
          ></AceEditorTheme>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            name="submitProfile"
            :value="t('buttons.update')"
          />
        </div>
      </form>
    </div>

    <div v-if="!noAuth" class="column">
      <section class="card profile-security-actions">
        <div v-if="!authStore.user?.lockPassword" class="card-title">
          <h2>{{ t("settings.changePassword") }}</h2>
        </div>
        <div v-if="!authStore.user?.lockPassword" class="card-action">
          <button
            v-if="!authStore.user?.lockPassword"
            class="button button--flat"
            type="button"
            @click="passwordModalOpen = true"
          >
            {{ t("settings.changePassword") }}
          </button>
        </div>
        <div class="card-title">
          <h2>{{ t("settings.multiFactorAuthentication") }}</h2>
        </div>
        <div class="card-action">
          <button
            class="button button--flat"
            type="button"
            @click="mfaModalOpen = true"
          >
            {{ t("settings.multiFactorAuthentication") }}
          </button>
        </div>
      </section>
    </div>
  </div>

  <base-modal v-if="passwordModalOpen" @closed="closePasswordModal">
    <form class="card floating profile-modal" @submit="updatePassword">
      <div class="card-title">
        <h2>{{ t("settings.changePassword") }}</h2>
      </div>
      <div class="card-content">
        <input
          id="focus-prompt"
          :class="passwordClass"
          type="password"
          :placeholder="t('settings.newPassword')"
          v-model="password"
          name="password"
          required
        />
        <input
          :class="passwordClass"
          type="password"
          :placeholder="t('settings.newPasswordConfirm')"
          v-model="passwordConf"
          name="passwordConf"
          required
        />
        <input
          v-if="isCurrentPasswordRequired"
          class="input input--block"
          type="password"
          :placeholder="t('settings.currentPassword')"
          v-model="currentPassword"
          name="current_password"
          autocomplete="current-password"
          required
        />
      </div>
      <div class="card-action">
        <button
          class="button button--flat button--grey"
          type="button"
          @click="closePasswordModal"
        >
          {{ t("buttons.cancel") }}
        </button>
        <button class="button button--flat" type="submit">
          {{ t("buttons.update") }}
        </button>
      </div>
    </form>
  </base-modal>

  <base-modal v-if="mfaModalOpen" @closed="closeMFAModal">
    <form class="card floating profile-modal" @submit.prevent="enableTOTP">
      <div class="card-title">
        <h2>{{ t("settings.multiFactorAuthentication") }}</h2>
      </div>
      <div class="card-content">
        <p>{{ t("settings.mfaDescription") }}</p>
        <input
          id="focus-prompt"
          class="input input--block"
          type="password"
          autocomplete="current-password"
          v-model="mfaPassword"
          :placeholder="t('settings.currentPassword')"
        />
        <template v-if="totpSetup === ''">
          <button class="button button--flat" type="submit">
            {{ t("settings.enableTotp") }}
          </button>
        </template>
        <template v-else>
          <qrcode-vue :value="totpSetup" :size="256" level="M" />
          <p class="mfa-setup-uri">{{ totpSetup }}</p>
          <input
            class="input input--block"
            inputmode="numeric"
            maxlength="6"
            v-model="totpCode"
            :placeholder="t('login.totpCode')"
          />
          <button
            class="button button--flat"
            type="button"
            @click="confirmTOTP"
          >
            {{ t("login.verifyTotp") }}
          </button>
        </template>
        <button class="button button--flat" type="button" @click="addPasskey">
          {{ t("settings.addPasskey") }}
        </button>
        <p v-if="mfaStatus.totp">{{ t("settings.totpEnabled") }}</p>
        <p v-if="mfaStatus.passkeys > 0">
          {{ t("settings.passkeysRegistered", { count: mfaStatus.passkeys }) }}
        </p>
      </div>
      <div class="card-action">
        <button
          class="button button--flat button--grey"
          type="button"
          @click="closeMFAModal"
        >
          {{ t("buttons.close") }}
        </button>
      </div>
    </form>
  </base-modal>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { users as api } from "@/api";
import * as mfa from "@/api/mfa";
import * as auth from "@/utils/auth";
import AceEditorTheme from "@/components/settings/AceEditorTheme.vue";
import Languages from "@/components/settings/Languages.vue";
import BaseModal from "@/components/prompts/BaseModal.vue";
import QrcodeVue from "qrcode.vue";
import { computed, inject, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { authMethod, noAuth } from "@/utils/constants";

const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const { t } = useI18n();

const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const $showError = inject<IToastError>("$showError")!;

const password = ref<string>("");
const passwordConf = ref<string>("");
const currentPassword = ref<string>("");
const isCurrentPasswordRequired = ref<boolean>(authMethod === "json");
const hideDotfiles = ref<boolean>(false);
const singleClick = ref<boolean>(false);
const redirectAfterCopyMove = ref<boolean>(false);
const dateFormat = ref<boolean>(false);
const locale = ref<string>("");
const aceEditorTheme = ref<string>("");
const mfaPassword = ref<string>("");
const totpSetup = ref<string>("");
const totpTransaction = ref<string>("");
const totpCode = ref<string>("");
const mfaStatus = ref<mfa.MFAStatus>({ totp: false, passkeys: 0 });
const passwordModalOpen = ref<boolean>(false);
const mfaModalOpen = ref<boolean>(false);

const showMFAError = (error: unknown) => {
  if (error instanceof Error && error.message === "passkey_failed") {
    $showError(t("errors.passkeyFailed"));
    return;
  }
  $showError(error instanceof Error ? error : String(error));
};

const passwordClass = computed(() => {
  const baseClass = "input input--block";

  if (password.value === "" && passwordConf.value === "") {
    return baseClass;
  }

  if (password.value === passwordConf.value) {
    return `${baseClass} input--green`;
  }

  return `${baseClass} input--red`;
});

const syncUserData = () => {
  if (authStore.user === null) return;
  locale.value = authStore.user.locale;
  hideDotfiles.value = authStore.user.hideDotfiles;
  singleClick.value = authStore.user.singleClick;
  redirectAfterCopyMove.value = authStore.user.redirectAfterCopyMove;
  dateFormat.value = authStore.user.dateFormat;
  aceEditorTheme.value = authStore.user.aceEditorTheme;
};

watch(() => authStore.user, syncUserData, { immediate: true });

onMounted(async () => {
  layoutStore.loading = true;
  isCurrentPasswordRequired.value = authMethod === "json";
  syncUserData();
  try {
    mfaStatus.value = await mfa.getStatus();
  } catch (error) {
    showMFAError(error);
  }
  layoutStore.loading = false;
});

const enableTOTP = async () => {
  try {
    const setup = await mfa.beginTOTP(mfaPassword.value);
    totpSetup.value = setup.uri;
    totpTransaction.value = setup.transaction;
  } catch (error) {
    showMFAError(error);
  }
};

const confirmTOTP = async () => {
  try {
    await mfa.confirmTOTP(totpTransaction.value, totpCode.value);
    mfaStatus.value = await mfa.getStatus();
    totpSetup.value = "";
    totpTransaction.value = "";
    totpCode.value = "";
    $showSuccess(t("settings.totpEnabled"));
  } catch (error) {
    showMFAError(error);
  }
};

const addPasskey = async () => {
  try {
    await mfa.enrollPasskey(mfaPassword.value);
    mfaStatus.value = await mfa.getStatus();
    $showSuccess(t("settings.passkeyAdded"));
  } catch (error) {
    showMFAError(error);
  }
};

const closePasswordModal = () => {
  passwordModalOpen.value = false;
  password.value = "";
  passwordConf.value = "";
  currentPassword.value = "";
};

const closeMFAModal = () => {
  mfaModalOpen.value = false;
  mfaPassword.value = "";
  totpSetup.value = "";
  totpTransaction.value = "";
  totpCode.value = "";
};

const updatePassword = async (event: Event) => {
  event.preventDefault();

  if (!password.value) {
    $showError(t("errors.emptyPassword"));
    return;
  }

  if (password.value !== passwordConf.value) {
    $showError(t("login.passwordsDontMatch"));
    return;
  }

  if (isCurrentPasswordRequired.value && !currentPassword.value) {
    $showError(t("errors.enterCurrentPassword"));
    return;
  }

  if (authStore.user === null) {
    return;
  }

  try {
    const data = {
      ...authStore.user,
      id: authStore.user.id,
      password: password.value,
    };
    await api.update(
      data,
      ["password"],
      isCurrentPasswordRequired.value ? currentPassword.value : ""
    );

    // Re-authenticate with new password so the session isn't invalidated
    try {
      await auth.login(authStore.user.username, password.value, "");
      authStore.updateUser(data);
      $showSuccess(t("settings.passwordUpdated"));
    } catch {
      $showSuccess(t("settings.passwordUpdated"));
      auth.logout();
    }
    closePasswordModal();
  } catch (e: any) {
    if (e instanceof Error) {
      const match = e.message.match(/minimum length is (\d+)/);
      if (match) {
        $showError(t("login.passwordTooShort", { min: match[1] }));
      } else if (e.message.includes("current password is incorrect")) {
        $showError(t("errors.currentPasswordIncorrect"));
      } else {
        $showError(e);
      }
    } else {
      $showError(e);
    }
  } finally {
    password.value = "";
    passwordConf.value = "";
    currentPassword.value = "";
  }
};
const updateSettings = async (event: Event) => {
  event.preventDefault();

  try {
    if (authStore.user === null) throw new Error("User is not set!");

    const data = {
      ...authStore.user,
      id: authStore.user.id,
      locale: locale.value,
      hideDotfiles: hideDotfiles.value,
      singleClick: singleClick.value,
      redirectAfterCopyMove: redirectAfterCopyMove.value,
      dateFormat: dateFormat.value,
      aceEditorTheme: aceEditorTheme.value,
    };

    await api.update(data, [
      "locale",
      "hideDotfiles",
      "singleClick",
      "redirectAfterCopyMove",
      "dateFormat",
      "aceEditorTheme",
    ]);
    authStore.updateUser(data);
    $showSuccess(t("settings.settingsUpdated"));
  } catch (err) {
    if (err instanceof Error) {
      $showError(err);
    }
  }
};
</script>

<style scoped>
.profile-security-actions .card-action + .card-title {
  border-top: 1px solid var(--borderPrimary);
}

.profile-modal {
  overflow: auto;
}

.mfa-setup-uri {
  overflow-wrap: anywhere;
  word-break: break-word;
}
</style>
