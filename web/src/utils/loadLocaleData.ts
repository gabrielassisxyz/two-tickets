// The locale is locked to pt-BR: there is no picker and no other locale is
// loaded. Empty pt-BR values are dropped so the English defaultMessage
// renders instead of a blank label (react-intl treats "" as a translation).
export const loadLocaleData = async (): Promise<Record<string, string>> => {
  const raw = (await import('../i18n/locale/pt_BR.json')).default as Record<
    string,
    string
  >;
  const messages: Record<string, string> = {};
  for (const [key, value] of Object.entries(raw)) {
    if (value !== '') {
      messages[key] = value;
    }
  }
  return messages;
};
