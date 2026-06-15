import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi'

export default createVuetify({
  icons: { defaultSet: 'mdi', aliases, sets: { mdi } },
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        colors: {
          primary: '#00838F', // cyan darken-3
          secondary: '#212121', // grey darken-4
          accent: '#546E7A', // blue-grey darken-1
          error: '#B71C1C' // red darken-4
        }
      }
    }
  }
})
