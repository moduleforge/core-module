import { GlobalRegistrator } from '@happy-dom/global-registrator';

// `disableCSSFileLoading` + `handleDisabledFileLoadingAsSuccess` keep tests
// deterministic and offline-safe: without them, happy-dom actually attempts
// a real network fetch for any `<link rel="stylesheet">` a test injects
// (e.g. the runtime theme-loader's tests), which is exactly the
// non-repeatable-network-call pattern node-design-standards' Testing section
// warns against, and would hang/error in a network-isolated CI environment.
GlobalRegistrator.register({
  settings: {
    disableCSSFileLoading: true,
    handleDisabledFileLoadingAsSuccess: true,
  },
});
