// Symbol + display-name lookup for ISO 4217 currency codes.
//
// The backend's `/api/v1/currencies` endpoint returns just an array of
// codes — symbols and names are FE-side metadata so the combobox can
// surface "$  USD — US Dollar" rows instead of bare codes (mock
// design-mocks/src/data/mock.ts L511-L542). Codes not listed here
// fall back to the bare code as both symbol and name.
//
// Keep this table in sync with the mock's CURRENCIES list. When a new
// currency lands on the BE side, append the matching entry here so
// the combobox keeps its rich rows.

export interface CurrencyMeta {
  code: string
  symbol: string
  name: string
}

const ENTRIES: readonly CurrencyMeta[] = [
  { code: "USD", symbol: "$", name: "US Dollar" },
  { code: "EUR", symbol: "€", name: "Euro" },
  { code: "GBP", symbol: "£", name: "British Pound" },
  { code: "CZK", symbol: "Kč", name: "Czech Koruna" },
  { code: "CHF", symbol: "Fr", name: "Swiss Franc" },
  { code: "JPY", symbol: "¥", name: "Japanese Yen" },
  { code: "CAD", symbol: "CA$", name: "Canadian Dollar" },
  { code: "AUD", symbol: "A$", name: "Australian Dollar" },
  { code: "SEK", symbol: "kr", name: "Swedish Krona" },
  { code: "NOK", symbol: "kr", name: "Norwegian Krone" },
  { code: "DKK", symbol: "kr", name: "Danish Krone" },
  { code: "PLN", symbol: "zł", name: "Polish Zloty" },
  { code: "HUF", symbol: "Ft", name: "Hungarian Forint" },
  { code: "RON", symbol: "lei", name: "Romanian Leu" },
  { code: "BGN", symbol: "лв", name: "Bulgarian Lev" },
  { code: "HRK", symbol: "kn", name: "Croatian Kuna" },
  { code: "RUB", symbol: "₽", name: "Russian Ruble" },
  { code: "CNY", symbol: "¥", name: "Chinese Yuan" },
  { code: "KRW", symbol: "₩", name: "South Korean Won" },
  { code: "INR", symbol: "₹", name: "Indian Rupee" },
  { code: "BRL", symbol: "R$", name: "Brazilian Real" },
  { code: "MXN", symbol: "MX$", name: "Mexican Peso" },
  { code: "ZAR", symbol: "R", name: "South African Rand" },
  { code: "SGD", symbol: "S$", name: "Singapore Dollar" },
  { code: "NZD", symbol: "NZ$", name: "New Zealand Dollar" },
  { code: "HKD", symbol: "HK$", name: "Hong Kong Dollar" },
  { code: "TWD", symbol: "NT$", name: "Taiwan Dollar" },
  { code: "TRY", symbol: "₺", name: "Turkish Lira" },
  { code: "SAR", symbol: "﷼", name: "Saudi Riyal" },
  { code: "AED", symbol: "د.إ", name: "UAE Dirham" },
]

const BY_CODE = new Map(ENTRIES.map((e) => [e.code, e]))

export function currencyMeta(code: string): CurrencyMeta {
  const upper = code.trim().toUpperCase()
  return BY_CODE.get(upper) ?? { code: upper, symbol: upper, name: upper }
}
