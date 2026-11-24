import { type AvailableLocale } from '@app/compat/types/languages';
import React from 'react';

type AvailableLanguageObject = Record<
  string,
  { code: AvailableLocale; display: string }
>;

export const availableLanguages: AvailableLanguageObject = {
  'pt-BR': {
    code: 'pt-BR',
    display: 'Português (Brasil)',
  },
};

export interface LanguageContextProps {
  locale: AvailableLocale;
  children: (locale: string) => React.ReactNode;
  setLocale?: React.Dispatch<React.SetStateAction<AvailableLocale>>;
}

export const LanguageContext = React.createContext<
  Omit<LanguageContextProps, 'children'>
>({
  locale: 'pt-BR',
});
