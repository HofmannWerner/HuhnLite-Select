const { createApp, ref, onMounted } = Vue;

const translations = {
  de: {
    title: 'HuhnLite | Mandantenauswahl',
    tabTitle: 'HuhnLite - Mandantenauswahl',
    refreshTooltip: 'Liste aktualisieren',
    welcomeTitle: 'Willkommen bei HuhnLite',
    welcomeSubtitle: 'Bitte wählen Sie einen Mandanten aus, um zu starten',
    active: 'Aktiv',
    inactive: 'Inaktiv',
    connect: 'Verbinden',
    startOpen: 'Starten & Öffnen',
    redirecting: 'Leite weiter zum Mandanten...',
    starting: "Mandant '{name}' wird gestartet...",
    started: 'Server gestartet! Verbinde...',
    errorStarting: 'Fehler beim Starten',
    errorDetail: 'Server konnte nicht gestartet werden.',
    networkError: 'Netzwerkfehler',
    networkErrorDetail: 'Keine Verbindung zum Launcher-Server.',
    close: 'Schließen',
    stillStarting: 'Server startet noch...',
    defaultDesc: 'Mandant Nr. {id}',
    mandantLabel: 'Mandant {id}',
    exitTooltip: 'HuhnLite-Select beenden',
    exitTitle: 'HuhnLite-Select beenden',
    exitConfirmMessage: 'Möchten Sie HuhnLite-Select wirklich beenden?',
    exitClosed: 'HuhnLite-Select wurde beendet. Sie können diesen Tab jetzt schließen.',
    exitTabOnlyMessage: 'Tab geschlossen. HuhnLite-Select bleibt im Hintergrund für andere aktive Sitzungen/Mandanten aktiv.',
    cancel: 'Abbrechen',
    exit: 'Beenden'
  },
  en: {
    title: 'HuhnLite | Client Selection',
    tabTitle: 'HuhnLite - Client Selection',
    refreshTooltip: 'Refresh list',
    welcomeTitle: 'Welcome to HuhnLite',
    welcomeSubtitle: 'Please select a client to start',
    active: 'Active',
    inactive: 'Inactive',
    connect: 'Connect',
    startOpen: 'Start & Open',
    redirecting: 'Redirecting to client...',
    starting: "Starting client '{name}'...",
    started: 'Server started! Connecting...',
    errorStarting: 'Error starting',
    errorDetail: 'Server could not be started.',
    networkError: 'Network error',
    networkErrorDetail: 'No connection to the launcher server.',
    close: 'Close',
    stillStarting: 'Server is still starting...',
    defaultDesc: 'Client No. {id}',
    mandantLabel: 'Client {id}',
    exitTooltip: 'Quit HuhnLite-Select',
    exitTitle: 'Quit HuhnLite-Select',
    exitConfirmMessage: 'Do you really want to quit HuhnLite-Select?',
    exitClosed: 'HuhnLite-Select has been shut down. You may now close this tab.',
    exitTabOnlyMessage: 'Tab closed. HuhnLite-Select remains active in the background for other active sessions/clients.',
    cancel: 'Cancel',
    exit: 'Quit'
  },
  it: {
    title: 'HuhnLite | Selezione del Mandante',
    tabTitle: 'HuhnLite - Selezione del Mandante',
    refreshTooltip: 'Aggiorna lista',
    welcomeTitle: 'Benvenuto su HuhnLite',
    welcomeSubtitle: 'Seleziona un mandante per iniziare',
    active: 'Attivo',
    inactive: 'Inattivo',
    connect: 'Connetti',
    startOpen: 'Avvia e Apri',
    redirecting: 'Reindirizzamento al mandante in corso...',
    starting: "Avvio del mandante '{name}' in corso...",
    started: 'Server avviato! Connessione in corso...',
    errorStarting: "Errore durante l'avvio",
    errorDetail: 'Impossibile avviare il server.',
    networkError: 'Errore di rete',
    networkErrorDetail: 'Nessuna connessione al server di avvio.',
    close: 'Chiudi',
    stillStarting: 'Il server si sta ancora avviando...',
    defaultDesc: 'Mandante N. {id}',
    mandantLabel: 'Mandante {id}',
    exitTooltip: 'Chiudi HuhnLite-Select',
    exitTitle: 'Chiudi HuhnLite-Select',
    exitConfirmMessage: 'Vuoi davvero uscire da HuhnLite-Select?',
    exitClosed: 'HuhnLite-Select è stato chiuso. Ora puoi chiudere questa scheda.',
    exitTabOnlyMessage: 'Scheda chiusa. HuhnLite-Select rimane attivo in background per altre sessioni/mandanti attivi.',
    cancel: 'Annulla',
    exit: 'Esci'
  }
};

const app = createApp({
  setup() {
    const mandanten = ref([]);
    const loading = ref(false);
    const startingDialog = ref(false);
    const selectedMandant = ref(null);
    const startingStatusMessage = ref('');
    const errorMessage = ref('');

    // Language state
    const currentLanguage = ref(localStorage.getItem('huhnlite-lang') || 'de');

    const t = (key, params = {}) => {
      const lang = currentLanguage.value;
      let text = translations[lang]?.[key] || translations['de']?.[key] || key;
      for (const [k, v] of Object.entries(params)) {
        text = text.replace(`{${k}}`, v);
      }
      return text;
    };

    const getMandantDisplayTitle = (m) => {
      if (!m) return '';
      if (m.name && !m.name.startsWith('Mandant ')) {
        return m.name;
      }
      return m.name || t('mandantLabel', { id: m.mandantNr || m.id });
    };

    const setLanguage = (lang) => {
      if (translations[lang]) {
        currentLanguage.value = lang;
        localStorage.setItem('huhnlite-lang', lang);
        document.title = t('tabTitle');
      }
    };

    // Dark Mode state
    const darkMode = ref(localStorage.getItem('huhnlite-dark') !== 'false');

    const toggleDarkMode = () => {
      darkMode.value = !darkMode.value;
      localStorage.setItem('huhnlite-dark', darkMode.value);
      Quasar.Dark.set(darkMode.value);
    };

    const loadMandanten = async () => {
      loading.value = true;
      try {
        const res = await fetch('/api/mandanten');
        if (res.ok) {
          const data = await res.json();
          mandanten.value = data || [];
        }
      } catch (err) {
        console.error('Fehler beim Laden der Mandanten:', err);
      } finally {
        loading.value = false;
      }
    };

    const getMandantUrl = (port) => {
      const protocol = window.location.protocol;
      const hostname = window.location.hostname;
      return `${protocol}//${hostname}:${port}`;
    };

    const selectMandant = async (mandant) => {
      selectedMandant.value = mandant;
      errorMessage.value = '';
      startingDialog.value = true;

      if (mandant.online) {
        startingStatusMessage.value = t('redirecting');
        setTimeout(() => {
          window.location.href = getMandantUrl(mandant.port);
        }, 500);
        return;
      }

      startingStatusMessage.value = t('starting', { name: mandant.name });
      
      try {
        const lang = currentLanguage.value || 'de';
        const isDark = darkMode.value ? 'true' : 'false';
        const res = await fetch(`/api/start?mandantId=${mandant.id}&lng=${lang}&lang=${lang}&language=${lang}&darkmode=${isDark}&dark=${isDark}`);
        const data = await res.json();

        if (res.ok && data.success) {
          startingStatusMessage.value = t('started');
          setTimeout(() => {
            const redirectUrl = (data.url && !data.url.includes('localhost')) ? data.url : getMandantUrl(mandant.port);
            window.location.href = redirectUrl;
          }, 800);
        } else {
          startingStatusMessage.value = t('errorStarting');
          errorMessage.value = data.message || t('errorDetail');
        }
      } catch (err) {
        startingStatusMessage.value = t('networkError');
        errorMessage.value = t('networkErrorDetail');
      }
    };

    const sessionId = sessionStorage.getItem('huhnlite-session-id') || 'sess_' + Math.random().toString(36).substring(2) + Date.now().toString(36);
    sessionStorage.setItem('huhnlite-session-id', sessionId);

    const sendHeartbeat = async () => {
      try {
        await fetch(`/api/heartbeat?sessionId=${sessionId}`);
      } catch (e) {
        // ignore network error
      }
    };

    const exitedDialog = ref(false);
    const exitStatusMessage = ref('');

    const closeTab = () => {
      // 1. Standard window.close
      window.close();

      // 2. Trick für Chromium-Browser (Chrome/Edge)
      try {
        window.open('', '_self', '');
        window.close();
      } catch (e) {}

      // 3. Fallback: Falls der Browser das Schließen von Tabs blockiert,
      // leite auf eine leere Seite (about:blank) weiter
      setTimeout(() => {
        if (!window.closed) {
          window.location.href = 'about:blank';
        }
      }, 250);
    };

    const confirmExit = () => {
      Quasar.Dialog.create({
        title: t('exitTitle'),
        message: t('exitConfirmMessage'),
        cancel: {
          label: t('cancel'),
          flat: true
        },
        ok: {
          label: t('exit'),
          color: 'negative',
          unelevated: true
        },
        dark: darkMode.value
      }).onOk(() => {
        exitedDialog.value = true;
        exitStatusMessage.value = t('exitClosed');
        try {
          fetch('/api/exit', { method: 'POST' });
        } catch (e) {}

        setTimeout(() => {
          closeTab();
        }, 300);
      });
    };

    onMounted(() => {
      Quasar.Dark.set(darkMode.value);
      document.title = t('tabTitle');
      loadMandanten();
      sendHeartbeat();
      setInterval(loadMandanten, 10000);
      setInterval(sendHeartbeat, 5000);
    });

    return {
      mandanten,
      loading,
      startingDialog,
      exitedDialog,
      exitStatusMessage,
      selectedMandant,
      startingStatusMessage,
      errorMessage,
      currentLanguage,
      darkMode,
      t,
      setLanguage,
      toggleDarkMode,
      getMandantDisplayTitle,
      loadMandanten,
      selectMandant,
      confirmExit
    };

  }
});

app.use(Quasar, {
  config: {
    brand: {
      primary: '#3b82f6',
    }
  }
});

app.mount('#q-app');
