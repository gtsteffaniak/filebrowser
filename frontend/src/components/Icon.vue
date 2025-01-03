<template>
  <span>
    <!-- Material Icon -->
    <i v-if="isMaterialIcon" :class="[classes, { active: active }]" class="icon"> {{ materialIcon }} </i>
  </span>
</template>

<script>
import { getTypeInfo } from "@/utils/mimetype";

export default {
  name: "Icon",
  props: {
    mimetype: {
      type: String,
      required: true,
    },
    active: {
      type: Boolean,
    }
  },
  data() {
    return {
      materialIcon: "",
      classes: "",
      svgPath: "",
    };
  },
  computed: {
    isMaterialIcon() {
      return this.materialIcon !== "";
    },
  },
  methods: {
    getIconForType(mimetype) {
      return getTypeInfo(mimetype);
    },
  },
  mounted() {
    const result = this.getIconForType(this.mimetype);
    this.classes = result.classes || "material-icons"; // Default class
    this.color = result.color || "lightgray"; // Default color
    this.materialIcon = result.materialIcon || "";
    this.svgPath = result.svgPath || ""; // For SVG file paths
  },
};
</script>

<style scoped>
.file-icons [aria-label^="."] {
  opacity: 0.33;
}

.file-icons [aria-label$=".bak"] {
  opacity: 0.33;
}

.svg-icons {
  display: flex;
  max-width: 100px;
}

.icon {
  font-size: 1.5rem;
  /* Default size */
  fill: currentColor;
  /* Uses inherited color */
  border-radius: 0.2em;
  padding: .1em;
  background: var(--alt-background);
}

.purple-icons {
  color: purple;
}

/* Icon Colors */
.blue-icons {
  color: var(--icon-blue);
}

/* Icon Colors */
.primary-icons {
  color: var(--primaryColor);
}

.primary-icons.active {
  text-shadow: 0px 0px 1px #000;
}


.lightblue-icons {
  color: lightskyblue;
}

.orange-icons {
  color: lightcoral;
}

.tan-icons {
  color: tan;
}

.plum-icons {
  color: plum;
}

.red-icons {
  color: rgb(246, 70, 70);
}

.beige-icons {
  color: beige;
}

.deep-blue-icons {
  color: rgb(29, 95, 191);
}

.green-icons {
  color: rgb(23, 128, 74);
}

.red-orange-icons {
  color: rgb(255, 147, 111);
}

.gray-icons {
  color: gray;
}

.skyblue-icons {
  color: rgb(42, 170, 242);
}

.lightgray-icons {
  color: rgb(176, 176, 176);
}

.yellow-icons {
  color: yellow;
}
</style>
