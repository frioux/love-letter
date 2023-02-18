<script lang="ts">
  import debounce from 'lodash/debounce';

  let s: string = "example";

   const handleInput = debounce(e => {
      s = e.target.value;
   }, 1000)
</script>

Rendering: {s}<br>
<br>
<img alt="render of {s}" src="/render/?s={s}" />

<form action="/save/" method="POST">
  <input on:input={handleInput} name="s" />
  <input value="save" type="submit" />
</form>
