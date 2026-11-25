import { useTranslation } from "react-i18next"

import type { ReminderRepeatUnit } from "@/features/reminders/api"

// useReminderText formats the parts of a reminder shared by the item page
// and the Upcoming page. Every branch is a literal t() call so the i18n
// extractor sees the keys.
export function useReminderText() {
  const { t } = useTranslation(["reminders"])

  function repeat(unit: ReminderRepeatUnit | undefined, interval: number | undefined): string {
    const count = interval && interval > 0 ? interval : 1
    switch (unit) {
      case "day":
        return count === 1 ? t("reminders:every.daily") : t("reminders:every.day", { count })
      case "week":
        return count === 1 ? t("reminders:every.weekly") : t("reminders:every.week", { count })
      case "month":
        return count === 1 ? t("reminders:every.monthly") : t("reminders:every.month", { count })
      case "year":
        return count === 1 ? t("reminders:every.yearly") : t("reminders:every.year", { count })
      default:
        return t("reminders:every.none")
    }
  }

  function leadTimes(days: number[]): string {
    if (days.length === 0) return ""
    const sorted = [...days].sort((a, b) => b - a)
    const before = sorted.filter((d) => d > 0)
    const parts: string[] = []
    if (before.length === 1) {
      parts.push(t("reminders:leadTimes.before", { list: String(before[0]), count: before[0] }))
    } else if (before.length > 1) {
      // A list agrees with its last number in cs/ru; English always uses
      // the plural here ("7, 1 days before").
      parts.push(
        t("reminders:leadTimes.beforeList", {
          list: before.join(", "),
          count: before[before.length - 1],
        })
      )
    }
    if (sorted.includes(0)) parts.push(t("reminders:leadTimes.onTheDay"))
    return t("reminders:leadTimes.prefix", { parts: parts.join(", ") })
  }

  function daysLeft(days: number): string {
    if (days === 0) return t("reminders:days.today")
    if (days < 0) return t("reminders:days.ago", { count: -days })
    return t("reminders:days.left", { count: days })
  }

  return { repeat, leadTimes, daysLeft }
}
