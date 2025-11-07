import HeaderBar from '../components/landing/HeaderBar'
import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { t } from '../i18n/translations'

/**
 * FAQ Page
 *
 * This page is just a collection of components, responsible for:
 * - Assembling HeaderBar and FAQLayout
 * - Providing global state (language, user, system configuration)
 * - Handling page-level navigation
 *
 * All FAQ-related logic is in child components:
 * - FAQLayout: overall layout and search logic
 * - FAQSearchBar: search bar
 * - FAQSidebar: left sidebar
 * - FAQContent: right content area
 *
 * FAQ data configuration is in data/faqData.ts
 */
export function FAQPage() {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const { config: systemConfig } = useSystemConfig()

  return (
    <div
      className="min-h-screen"
      style={{ background: '#000000', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage="faq"
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        isAdminMode={systemConfig?.admin_mode}
        onPageChange={(page) => {
          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            window.location.href = '/competition'
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            window.location.href = '/traders'
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            window.location.href = '/dashboard'
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            window.location.href = '/faq'
          }
        }}
      />

      <FAQLayout language={language} />

      {/* Footer */}
      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <div
          className="max-w-7xl mx-auto px-6 py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </footer>
    </div>
  )
}
