<script lang="ts">
  import debounce from 'lodash/debounce';

  let s: string = "example";

   // generating the image takes about 1.1s so
   // picking half that as debounce time.
   const handleInput = debounce(e => {
      s = e.target.value;
   }, 550)
</script>

Rendering: {s}<br>
<br>
<img alt="render of {s}" src="/render/?s={s}" />

<form action="/save/" method="POST">
  <input on:input={handleInput} name="s" />
  <input value="save" type="submit" />
</form>
