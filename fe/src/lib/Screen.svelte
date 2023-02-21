<script lang="ts">
  import debounce from 'lodash/debounce';

  let s: string = "example";
  let enc_s: string;

  $: enc_s = encodeURIComponent(s);

   // generating the image takes about 1.1s so
   // picking half that as debounce time.
   const handleInput = debounce(e => {
      s = e.target.value;
   }, 550)
</script>

<style>
  /* Via https://stackoverflow.com/a/45019339 */
  .my-img-container {
    position: relative;
    padding-top: 50%;
  }
  .my-img-container:before {
    content: " ";
    position: absolute;
    top: 50%;
    left: 50%;
    width: 80px;
    height: 80px;
    border: 2px solid white;
    border-color: transparent white transparent white;
    border-radius: 50%;
    animation: loader 1s linear infinite;
  }
  .my-img-container > img {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100% !important;
    height: 100% !important;
  }
  @keyframes loader {
    0% {
      transform: translate(-50%,-50%) rotate(0deg);
    }
    100% {
      transform: translate(-50%,-50%) rotate(360deg);
    }
  }
</style>

<div class="my-img-container">
   {#key s}
      <img alt="Rendering '{s}'" src="/render/?s={enc_s}" />
   {/key}
</div>

<form action="/save/" method="POST">
  <input on:input={handleInput} name="s" />
  <input value="save" type="submit" />
</form>
