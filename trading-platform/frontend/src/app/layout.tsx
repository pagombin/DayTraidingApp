import './globals.css'
import AppShell from '@/components/AppShell'

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen bg-gray-900 dark:bg-gray-900 text-white">
        <AppShell>{children}</AppShell>
      </body>
    </html>
  )
}
