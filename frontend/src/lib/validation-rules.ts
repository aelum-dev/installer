// Single source of truth for user-account validation rules.
//
// The rules mirror Linux shadow-utils 4.20 (useradd with NAME_REGEX unset, as on
// Arch): a name may be 1..255 characters, starts with a letter, digit,
// underscore, or dot, and otherwise contains only letters, digits, underscore,
// dot, or hyphen, plus an optional trailing '$'. "." and "..", leading '-', and
// all-digit names are rejected. The installer adds no extra case or punctuation
// policy.
//
// The account-name list is generated from Go (shared.ReservedAccountNames) so the
// inline validator and the backend can never drift apart.

import { ReservedAccountNames } from './constants';

// A Linux login name starts with a letter, digit, underscore, or dot.
export const USERNAME_FIRST_CHAR = /^[A-Za-z0-9_.]/;

// ...and otherwise contains only letters, digits, underscores, dots, and
// hyphens, with an optional trailing '$' (Samba machine accounts).
export const USERNAME_ALLOWED_CHARS = /^[A-Za-z0-9_.-]*\$?$/;

// Linux rejects names that are made up only of digits.
export const USERNAME_ALL_DIGITS = /^[0-9]+$/;

// Existing accounts and groups on a stock Aelum install. These names would
// collide with useradd's private-group creation or reuse a system account.
export const RESERVED_USERNAMES = new Set(ReservedAccountNames.split(','));
