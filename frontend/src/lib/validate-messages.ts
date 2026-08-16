import { t } from '$lib/i18n';
import type { ValidationState } from './validate';

export interface ValidationMessages {
  usernameRequired: string;
  usernameMinLength: string;
  usernameMaxLength: string;
  usernameInvalidFirstChar: string;
  usernameInvalidChars: string;
  usernameReserved: string;
  passwordRequired: string;
  passwordMinLength: string;
  passwordsMatch: string;
  passwordsNoMatch: string;
}

export function getValidationMessages(): ValidationMessages {
  return {
    usernameRequired: t('validation.usernameRequired'),
    usernameMinLength: t('validation.usernameMinLength'),
    usernameMaxLength: t('validation.usernameMaxLength'),
    usernameInvalidFirstChar: t('validation.usernameInvalidFirstChar'),
    usernameInvalidChars: t('validation.usernameInvalidChars'),
    usernameReserved: t('validation.usernameReserved'),
    passwordRequired: t('validation.passwordRequired'),
    passwordMinLength: t('validation.passwordMinLength'),
    passwordsMatch: t('validation.passwordsMatch'),
    passwordsNoMatch: t('validation.passwordsNoMatch'),
  };
}
