import type { GlobalProvider } from '@ladle/react';
import './styles.css';

// Migrated from the legacy `.dark`-class toggle onto the unified `data-mf-theme` scoping
// attribute (see ../tokens/CONTRACT.md). The Ladle theme addon still drives `globalState.theme`;
// we translate its light/dark state onto the attribute. The `.dark` class remains bridged in the
// compiled token CSS + `@custom-variant dark`, so any external consumer still toggling `.dark`
// keeps working.
export const Provider: GlobalProvider = ({ children, globalState }) => (
  <div data-mf-theme={globalState.theme === 'dark' ? 'dark' : 'light'}>
    <div className="min-h-screen bg-background text-foreground p-6">
      {children}
    </div>
  </div>
);
