import { motion } from 'framer-motion'
import AnimatedSection from './AnimatedSection'
import { t, type Language } from '../../i18n/translations'

interface CardProps {
  quote: string
  authorName: string
  handle: string
  avatarUrl: string
  tweetUrl: string
  delay: number
}

function TestimonialCard({ quote, authorName, delay }: CardProps) {
  return (
    <motion.div
      className="p-6 rounded-xl"
      style={{
        background: 'var(--brand-dark-gray)',
        border: '1px solid rgba(240, 185, 11, 0.1)',
      }}
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay }}
      whileHover={{ scale: 1.05 }}
    >
      <p className="text-lg mb-4" style={{ color: 'var(--brand-light-gray)' }}>
        "{quote}"
      </p>
      <div className="flex items-center gap-2">
        <div
          className="w-8 h-8 rounded-full"
          style={{ background: 'var(--binance-yellow)' }}
        />
        <span
          className="text-sm font-semibold"
          style={{ color: 'var(--text-secondary)' }}
        >
          {authorName}
        </span>
      </div>
    </motion.div>
  )
}

interface CommunitySectionProps {
  language: Language
}

export default function CommunitySection({ language }: CommunitySectionProps) {
  const staggerContainer = {
    animate: { transition: { staggerChildren: 0.1 } },
  }

  // Twitter content integration (maintain original three-column layout, auto-wrap on overflow)
  const items: CardProps[] = [
    {
      quote: t('testimonial1', language),
      authorName: 'Michael Williams',
      handle: '@MichaelWil93725',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1767615411594694659/Mj8Fdt6o_400x400.jpg',
      tweetUrl:
        'https://twitter.com/MichaelWil93725/status/1984980920395604008',
      delay: 0,
    },
    {
      quote: t('testimonial2', language),
      authorName: 'DIŸgöd',
      handle: '@DIYgod',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1628393369029181440/r23HDDJk_400x400.jpg',
      tweetUrl: 'https://twitter.com/DIYgod/status/1984442354515017923',
      delay: 0.1,
    },
    {
      quote: t('testimonial3', language),
      authorName: 'Kai',
      handle: '@hqmank',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1905441261911506945/4YhLIqUm_400x400.jpg',
      tweetUrl: 'https://twitter.com/hqmank/status/1984227431994290340',
      delay: 0.15,
    },
  ]

  return (
    <AnimatedSection>
      <div className="max-w-7xl mx-auto">
        <motion.div
          className="grid md:grid-cols-3 gap-6"
          variants={staggerContainer}
          initial="initial"
          whileInView="animate"
          viewport={{ once: true }}
        >
          {items.map((item, idx) => (
            <TestimonialCard key={idx} {...item} />
          ))}
        </motion.div>
      </div>
    </AnimatedSection>
  )
}
