---
phase: quick-001
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - src/components/profile/SectionHeader.tsx
  - src/components/profile/SettingsRow.tsx
  - src/components/profile/LocationPrivacySelector.tsx
  - src/components/profile/TripStats.tsx
  - src/features/users/api.ts
  - src/features/users/types.ts
  - app/(tabs)/profile.tsx
autonomous: true

must_haves:
  truths:
    - "User sees their avatar, name, username, and joined date at top of profile"
    - "User can select location privacy level (Hidden/Approximate/Precise)"
    - "User can toggle Ghost Mode on/off"
    - "User sees trip statistics (trips count, countries, days traveled)"
    - "User can navigate to settings sections via tappable rows"
    - "User can sign out and access delete account option"
  artifacts:
    - path: "src/components/profile/SectionHeader.tsx"
      provides: "Reusable section header component"
    - path: "src/components/profile/SettingsRow.tsx"
      provides: "Reusable settings row with icon, label, value, and chevron"
    - path: "src/components/profile/LocationPrivacySelector.tsx"
      provides: "Location privacy level selector component"
    - path: "src/components/profile/TripStats.tsx"
      provides: "Trip statistics display component"
    - path: "src/features/users/api.ts"
      provides: "User API functions for profile updates"
    - path: "app/(tabs)/profile.tsx"
      provides: "Complete profile screen implementation"
  key_links:
    - from: "app/(tabs)/profile.tsx"
      to: "src/features/users/api.ts"
      via: "updateUserPreferences, updateLocationPrivacy"
    - from: "app/(tabs)/profile.tsx"
      to: "useAuthStore"
      via: "user state and signOut"
---

<objective>
Implement the complete NomadCrew Profile Screen with Identity Hero, Location Privacy, Ghost Mode, Trip Stats, Settings, Account, and Support sections.

Purpose: Transform the minimal profile page into a full-featured user profile with privacy controls and navigation to settings.
Output: A polished profile screen with reusable components (SectionHeader, SettingsRow, LocationPrivacySelector, TripStats) and user API integration.
</objective>

<execution_context>
@~/.claude/get-shit-done/workflows/execute-plan.md
@~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
Working directory: N:\NomadCrew\nomad-crew-frontend

Key files to reference:
@app/(tabs)/profile.tsx - Current minimal profile (starting point)
@components/ui/Avatar.tsx - Existing Avatar component with sizes xs/sm/md/lg/xl
@src/features/auth/store.ts - useAuthStore with user state and signOut
@src/features/auth/types.ts - User type definition
@src/theme/foundations/colors.ts - Theme colors (colorTokens, createSemanticColors)
@src/components/ThemedText.tsx - ThemedText with variant and color props
@src/components/ThemedView.tsx - ThemedView component
@src/utils/api-paths.ts - API path definitions (users.me, users.byId)

Backend API endpoints:
- GET /v1/users/me - Returns user with locationPrivacyPreference field
- PUT /v1/users/{id} - Update user profile
- PUT /v1/users/{id}/preferences - Update preferences (JSON merge)

User.locationPrivacyPreference: 'hidden' | 'approximate' | 'precise'
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create reusable profile components</name>
  <files>
    src/components/profile/SectionHeader.tsx
    src/components/profile/SettingsRow.tsx
    src/components/profile/index.ts
  </files>
  <action>
Create reusable components for the profile screen:

**SectionHeader.tsx:**
- Props: `title: string`, `style?: ViewStyle`
- Uses ThemedText with variant "label.large" and color "content.secondary"
- Uppercase text, marginTop 24, marginBottom 8, paddingHorizontal 16
- Export as named export

**SettingsRow.tsx:**
- Props: `icon: string` (Ionicons name), `label: string`, `value?: string`, `onPress?: () => void`, `rightElement?: ReactNode`, `showChevron?: boolean` (default true)
- Pressable row with:
  - Left: Ionicons icon (size 22, color content.secondary)
  - Center: Label (body.medium) + optional value below (body.small, content.tertiary)
  - Right: Either rightElement OR chevron-forward icon (if showChevron && onPress)
- Height 56, paddingHorizontal 16, backgroundColor background.card
- Use `useThemedStyles` for theme access
- Add subtle border-bottom (1px, border.default)
- Export as named export

**index.ts:**
- Export both components
  </action>
  <verify>
- `npx tsc --noEmit` passes
- Files exist at specified paths
- Components export correctly
  </verify>
  <done>
SectionHeader and SettingsRow components created with proper typing, theming, and exports.
  </done>
</task>

<task type="auto">
  <name>Task 2: Create LocationPrivacySelector and TripStats components</name>
  <files>
    src/components/profile/LocationPrivacySelector.tsx
    src/components/profile/TripStats.tsx
    src/components/profile/index.ts
  </files>
  <action>
**LocationPrivacySelector.tsx:**
- Props: `value: 'hidden' | 'approximate' | 'precise'`, `onChange: (value) => void`, `disabled?: boolean`
- Three-option selector (prominent, not just a dropdown):
  - Hidden: "eye-off-outline" icon, "Hidden" label, "Location not shared"
  - Approximate: "location-outline" icon, "Approximate" label, "City-level only"
  - Precise: "navigate-outline" icon, "Precise" label, "Exact location"
- Display as horizontal button group OR vertical radio-style list
- Selected option: primary.main background with white text
- Unselected: background.card with content.primary text, border primary.border
- Use Pressable for each option
- Border radius theme.borderRadius.md
- Animate selection with subtle scale (0.98 on press)

**TripStats.tsx:**
- Props: `trips: number`, `countries: number`, `daysTraveled: number`
- Horizontal row with three stat items, evenly spaced
- Each stat: large number (display.small, primary.main) above label (body.small, content.secondary)
- Centered text for each stat
- Use flexDirection 'row', justifyContent 'space-around'
- paddingVertical 16, backgroundColor background.card, borderRadius md

Update **index.ts** to export new components.
  </action>
  <verify>
- `npx tsc --noEmit` passes
- Components render correctly when imported
- LocationPrivacySelector calls onChange when option tapped
  </verify>
  <done>
LocationPrivacySelector with three visual options and TripStats with formatted display created.
  </done>
</task>

<task type="auto">
  <name>Task 3: Create user API functions</name>
  <files>
    src/features/users/types.ts
    src/features/users/api.ts
    src/features/users/index.ts
  </files>
  <action>
**types.ts:**
```typescript
export type LocationPrivacyLevel = 'hidden' | 'approximate' | 'precise';

export interface UserPreferences {
  notifications?: {
    push?: boolean;
    email?: boolean;
    tripUpdates?: boolean;
    chatMessages?: boolean;
  };
  ghostMode?: boolean;
  // extensible for future preferences
  [key: string]: unknown;
}

export interface UserProfile {
  id: string;
  email: string;
  username: string;
  firstName?: string;
  lastName?: string;
  profilePictureUrl?: string;
  createdAt: string;
  updatedAt?: string;
  locationPrivacyPreference: LocationPrivacyLevel;
  preferences?: UserPreferences;
}

export interface UpdateUserProfilePayload {
  firstName?: string;
  lastName?: string;
  username?: string;
}

export interface UpdatePreferencesPayload {
  ghostMode?: boolean;
  notifications?: Partial<UserPreferences['notifications']>;
}
```

**api.ts:**
```typescript
import { api } from '@/src/api/api-client';
import { API_PATHS } from '@/src/utils/api-paths';
import type { UserProfile, LocationPrivacyLevel, UpdatePreferencesPayload } from './types';

export async function fetchUserProfile(): Promise<UserProfile> {
  const response = await api.get<UserProfile>(API_PATHS.users.me);
  return response.data;
}

export async function updateLocationPrivacy(
  userId: string,
  level: LocationPrivacyLevel
): Promise<UserProfile> {
  const response = await api.put<UserProfile>(API_PATHS.users.byId(userId), {
    locationPrivacyPreference: level,
  });
  return response.data;
}

export async function updateUserPreferences(
  userId: string,
  preferences: UpdatePreferencesPayload
): Promise<UserProfile> {
  // Uses merge-based preferences endpoint
  const response = await api.put<UserProfile>(
    `${API_PATHS.users.byId(userId)}/preferences`,
    preferences
  );
  return response.data;
}
```

Update **index.ts** to export types and API functions alongside existing UserAutocomplete.
  </action>
  <verify>
- `npx tsc --noEmit` passes
- Types are correctly exported
- API functions use correct endpoints from api-paths.ts
  </verify>
  <done>
User types and API functions created for profile management with locationPrivacyPreference and preferences support.
  </done>
</task>

<task type="auto">
  <name>Task 4: Implement complete Profile screen</name>
  <files>
    app/(tabs)/profile.tsx
  </files>
  <action>
Rewrite profile.tsx to implement the full profile screen:

**Imports:**
- React, useState, useCallback, useMemo
- ScrollView, StyleSheet, View, Alert, Linking
- useSafeAreaInsets, useFocusEffect
- ThemedView, ThemedText from src/components
- Avatar from components/ui/Avatar
- Button, Switch from react-native-paper
- Ionicons from @expo/vector-icons
- useAuthStore from src/features/auth/store
- SectionHeader, SettingsRow, LocationPrivacySelector, TripStats from src/components/profile
- updateLocationPrivacy, updateUserPreferences, fetchUserProfile from src/features/users
- useThemedStyles from src/theme/utils
- Constants from expo-constants (for app version)

**State:**
- `locationPrivacy: LocationPrivacyLevel` (init from user or 'hidden')
- `ghostMode: boolean` (init false)
- `isUpdating: boolean` (loading state)
- `tripStats: { trips: number, countries: number, daysTraveled: number }` (placeholder: { trips: 5, countries: 3, daysTraveled: 42 })

**Layout (ScrollView):**

1. **Identity Hero** (top section):
   - Avatar size 96px (custom style override on xl, or create new size)
   - Full name (display.small) - `${user.firstName} ${user.lastName}` or username
   - @username (body.medium, content.secondary)
   - "Joined {month year}" from user.createdAt (body.small, content.tertiary)
   - Online indicator: green dot (8px) with "Online" text if desired (optional)
   - Center aligned, paddingVertical 24

2. **Location Privacy Section** (PROMINENT):
   - SectionHeader: "LOCATION SHARING"
   - LocationPrivacySelector with value/onChange
   - On change: call updateLocationPrivacy(user.id, newValue), update local state
   - Show loading indicator during update

3. **Ghost Mode Section:**
   - SectionHeader: "GHOST MODE"
   - SettingsRow with Switch as rightElement:
     - icon: "moon-outline"
     - label: "Ghost Mode"
     - value: "Pause all location sharing temporarily"
     - rightElement: Switch with value={ghostMode}, onValueChange handler
   - On toggle: call updateUserPreferences(user.id, { ghostMode: newValue })

4. **Trip Stats Section:**
   - SectionHeader: "YOUR JOURNEY"
   - TripStats component with placeholder data
   - Note: Real data will come from trip API (future enhancement)

5. **Settings Section:**
   - SectionHeader: "SETTINGS"
   - SettingsRow: icon="notifications-outline", label="Notifications", onPress={navigateToNotifications}
   - SettingsRow: icon="options-outline", label="Preferences", onPress={navigateToPreferences}
   - For now, onPress can show Alert "Coming soon" or use router.push if routes exist

6. **Account Section:**
   - SectionHeader: "ACCOUNT"
   - SettingsRow: icon="mail-outline", label="Email", value={user.email}, showChevron=false
   - SettingsRow: icon="at-outline", label="Username", value={`@${user.username}`}, showChevron=false
   - SettingsRow: icon="link-outline", label="Connected Accounts", value="Apple, Google" (placeholder), onPress (future)

7. **Support Section:**
   - SectionHeader: "SUPPORT"
   - SettingsRow: icon="help-circle-outline", label="Help & FAQ", onPress={() => Linking.openURL('https://nomadcrew.uk/help')}
   - SettingsRow: icon="shield-checkmark-outline", label="Privacy Policy", onPress={() => Linking.openURL('https://nomadcrew.uk/privacy')}
   - SettingsRow: icon="document-text-outline", label="Terms of Service", onPress={() => Linking.openURL('https://nomadcrew.uk/terms')}
   - SettingsRow: icon="information-circle-outline", label="App Version", value={Constants.expoConfig?.version || '1.0.0'}, showChevron=false

8. **Sign Out & Delete:**
   - Button mode="contained" for Sign Out (margin 16, uses signOut from store)
   - ThemedText as Pressable for "Delete Account" (body.small, status.error.main color)
   - Delete Account shows Alert confirmation, then placeholder action

**Error handling:**
- Wrap API calls in try/catch
- Show Alert on error with user-friendly message
- Reset local state on error

**Styling:**
- Use useThemedStyles for all theme-dependent styles
- Consistent spacing (theme.spacing)
- Cards/sections with background.card background
- Proper safe area handling (bottom padding for scroll)
  </action>
  <verify>
- `npx tsc --noEmit` passes
- `npx expo start` runs without errors
- Profile screen displays all sections
- Location privacy selector changes value on tap
- Ghost mode toggle works
- Sign out button works
- Links open in browser
  </verify>
  <done>
Complete profile screen with Identity Hero, Location Privacy (prominent), Ghost Mode, Trip Stats, Settings, Account, Support sections, Sign Out, and Delete Account link.
  </done>
</task>

</tasks>

<verification>
1. TypeScript compiles without errors: `npx tsc --noEmit`
2. App runs: `npx expo start`
3. Profile tab shows complete new UI
4. Location privacy selector is visually prominent and functional
5. Ghost mode toggle updates (API call made)
6. All SettingsRows are tappable where applicable
7. Sign out works correctly
8. External links open in browser
</verification>

<success_criteria>
- Profile screen displays all 8 sections (Hero, Location Privacy, Ghost Mode, Stats, Settings, Account, Support, Actions)
- Location privacy is the PROMINENT feature with clear visual selection
- Reusable components (SectionHeader, SettingsRow, LocationPrivacySelector, TripStats) work correctly
- User API integration for preferences updates
- Clean, polished UI matching NomadCrew design language
- No TypeScript errors
</success_criteria>

<output>
After completion, create `.planning/quick/001-implement-nomadcrew-profile-screen/001-SUMMARY.md` with:
- Components created and their purposes
- API functions implemented
- Profile screen sections implemented
- Any deviations from plan
- Screenshots or verification results
</output>
