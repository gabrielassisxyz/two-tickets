interface PWAHeaderProps {
  applicationTitle?: string;
}

// Browser metadata only: no manifest, no service worker, no splash images.
// The app is not installable and works online only.
const PWAHeader = ({ applicationTitle = 'Two Tickets' }: PWAHeaderProps) => {
  return (
    <>
      <link
        rel="apple-touch-icon"
        sizes="180x180"
        href="/apple-touch-icon.png"
      />
      <link
        rel="icon"
        type="image/png"
        sizes="32x32"
        href="/favicon-32x32.png"
      />
      <link
        rel="icon"
        type="image/png"
        sizes="16x16"
        href="/favicon-16x16.png"
      />
      <meta name="description" content="Two Tickets" />
      <meta name="theme-color" content="#18181b" />
      <meta name="application-name" content={applicationTitle} />
    </>
  );
};

export default PWAHeader;
