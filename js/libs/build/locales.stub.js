// Bundle-time replacement for zod's full locale index (51 languages, ~196 KB
// minified). Only the locales scripts can actually select are kept; add a line
// here to ship another language.
export { default as en } from "zod/v4/locales/en.js";
export { default as zhCN } from "zod/v4/locales/zh-CN.js";
