import * as cc from 'currency-codes';

type CurrencyElement = { id: string, name: string };

export const CURRENCIES: string[] = cc.codes();
export const CURRENCIES_ELEMENTS: CurrencyElement[] = CURRENCIES.map((currency: string): CurrencyElement => ({
  id: currency,
  name: currency,
}));
