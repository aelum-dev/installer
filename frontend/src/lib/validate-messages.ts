import { t } from '$lib/i18n';
import type { ValidationMessages } from './validate';

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
