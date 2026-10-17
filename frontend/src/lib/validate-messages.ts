import { t } from '$lib/i18n';
import type { ValidationMessages } from './validate';

export function getValidationMessages(): ValidationMessages {
  return {
    usernameRequired: t('validation.usernameRequired'),
    usernameTooLong: t('validation.usernameTooLong'),
    usernameDotNames: t('validation.usernameDotNames'),
    usernameDigitsOnly: t('validation.usernameDigitsOnly'),
    usernameInvalidStart: t('validation.usernameInvalidStart'),
    usernameInvalidCharacters: t('validation.usernameInvalidCharacters'),
    usernameReserved: t('validation.usernameReserved'),
    passwordRequired: t('validation.passwordRequired'),
    passwordTooLong: t('validation.passwordTooLong'),
    passwordInvalidCharacters: t('validation.passwordInvalidCharacters'),
    passwordsMatch: t('validation.passwordsMatch'),
    passwordsNoMatch: t('validation.passwordsNoMatch'),
  };
}
