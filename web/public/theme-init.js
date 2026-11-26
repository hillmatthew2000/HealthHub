// Applies the saved or system colour scheme before the app renders, so the
// page does not flash the wrong theme. Kept as a file so the content
// security policy does not need to allow inline scripts.
;(function () {
  var pref = 'system'
  try {
    pref = localStorage.getItem('healthhub.theme') || 'system'
  } catch (e) {
    /* storage unavailable */
  }
  var dark = pref === 'dark' || (pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
})()
