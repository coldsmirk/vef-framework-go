// Entry for the vef js stdlib bundle. Each library is exposed under its
// ecosystem-native global name; integrating a new library is one import plus
// one global assignment here, then a rebuild (see build.js).
//
// The core-js imports are self-installing polyfills for web platform
// primitives goja lacks; they bind their own globals.
import "core-js/web/url";
import "core-js/web/url-search-params";

import BigNumber from "bignumber.js";
import dayjs from "dayjs";
import * as fxp from "fast-xml-parser";
import * as radashi from "radashi";
import * as z from "zod";

globalThis.BigNumber = BigNumber;
globalThis.dayjs = dayjs;
globalThis.fxp = fxp;
globalThis.radashi = radashi;
globalThis.z = z;

// The framework's default language is Simplified Chinese (VEF_I18N_LANGUAGE);
// scripts targeting English users can switch with z.config(z.locales.en()).
z.config(z.locales.zhCN());
