import { useLayoutEffect } from 'react';
import { useNavigation } from 'expo-router';
import { t, useI18n } from '../i18n';

/**
 * Sets the header title and Back label in the chosen language.
 * Titles set on <Stack.Screen> in _layout did not follow a language change (they stayed in
 * Russian after switching to English), so each screen sets its own and re-sets it on a change.
 */
export function useHeaderText(title?: string) {
  const navigation = useNavigation();
  const { locale } = useI18n();
  useLayoutEffect(() => {
    navigation.setOptions({ headerBackTitle: t('Back'), ...(title ? { title: t(title) } : {}) });
  }, [navigation, locale, title]);
}
