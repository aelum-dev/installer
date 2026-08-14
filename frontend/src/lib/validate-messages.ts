import { t } from '$lib/i18n';
import type { ValidationState } from './validate';

export interface ValidationMessages {
  usernameRequired: string;
  usernameMinLength: string;
  passwordRequired: string;
  passwordMinLength: string;
  passwordsMatch: string;
  passwordsNoMatch: string;
}

export function getValidationMessages(): ValidationMessages {
  return {
    usernameRequired: t('validation.usernameRequired'),
    usernameMinLength: t('validation.usernameMinLength'),
    passwordRequired: t('validation.passwordRequired'),
    passwordMinLength: t('validation.passwordMinLength'),
    passwordsMatch: t('validation.passwordsMatch'),
    passwordsNoMatch: t('validation.passwordsNoMatch'),
  };
}
