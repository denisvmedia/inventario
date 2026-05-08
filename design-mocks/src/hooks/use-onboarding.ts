import { useState, useEffect } from "react"

const STORAGE_KEY = "homelog_onboarding_completed"

export function useOnboarding() {
  const [active, setActive] = useState(false)
  const [step, setStep] = useState(0)

  useEffect(() => {
    const completed = localStorage.getItem(STORAGE_KEY)
    if (!completed) {
      // Small delay so layout settles
      const t = setTimeout(() => setActive(true), 600)
      return () => clearTimeout(t)
    }
  }, [])

  function next() {
    setStep((s) => s + 1)
  }

  function prev() {
    setStep((s) => Math.max(0, s - 1))
  }

  function finish() {
    localStorage.setItem(STORAGE_KEY, "1")
    setActive(false)
    setStep(0)
  }

  function skip() {
    finish()
  }

  function restart() {
    localStorage.removeItem(STORAGE_KEY)
    setStep(0)
    setActive(true)
  }

  return { active, step, next, prev, finish, skip, restart }
}
