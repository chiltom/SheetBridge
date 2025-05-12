document.addEventListener("DOMContentLoaded", () => {
  const dropZone = document.getElementById("dropZone");
  const fileInput = document.getElementById("fileInput");
  const loading = document.getElementById("loading");
  const optionsDiv = document.getElementById("options");
  const tableSelect = document.getElementById("tableSelect");
  const newTableName = document.getElementById("newTableName");
  const previewBtn = document.getElementById("previewBtn");
  const previewDiv = document.getElementById("preview");
  const previewTable = document.getElementById("previewTable");
  const commitBtn = document.getElementById("commitBtn");
  let csvFile;

  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("dragover");
  });

  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("dragover");
  });

  dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropZone.classList.remove("dragover");
    csvFile = e.dataTransfer.files[0];
    if (csvFile && csvFile.type === "text/csv") {
      uploadCSV(csvFile);
    } else {
      alert("Please upload a valid CSV file.");
    }
  });

  dropZone.addEventListener("click", () => fileInput.click());
  fileInput.addEventListener("change", () => {
    csvFile = fileInput.files[0];
    if (csvFile) {
      uploadCSV(csvFile);
    }
  });

  function showLoading(show) {
    loading.style.display = show ? "block" : "none";
  }

  function uploadCSV(file) {
    showLoading(true);
    const formData = new FormData();
    formData.append("csvfile", file);

    fetch("/upload", {
      method: "POST",
      body: formData,
    })
      .then((response) => {
        if (!response.ok) throw new Error("Failed to upload CSV");
        return response.json();
      })
      .then((data) => {
        tableSelect.innerHTML = '<option value="">-- Select Table --</option>';
        data.tableNames.forEach((name) => {
          const option = document.createElement("option");
          option.value = name;
          option.textContent = name;
          tableSelect.appendChild(option);
        });

        optionsDiv.style.display = "block";
        previewDiv.style.display = "none";
        showLoading(false);
      })
      .catch((error) => {
        showLoading(false);
        console.error("Error:", error);
        alert(`Error processing CSV: ${error.message}`);
      });
  }

  previewBtn.addEventListener("click", () => {
    if (!csvFile) {
      alert("Please upload a CSV file first.");
      return;
    }

    showLoading(true);
    const formData = new FormData();
    formData.append("csvfile", csvFile);

    fetch("/upload", {
      method: "POST",
      body: formData,
    })
      .then((response) => {
        if (!response.ok) throw new Error("Failed to generate preview");
        return response.json();
      })
      .then((data) => {
        const thead = previewTable.querySelector("thead");
        const tbody = previewTable.querySelector("tbody");
        thead.innerHTML = "";
        tbody.innerHTML = "";

        const headerRow = document.createElement("tr");
        data.headers.forEach((col) => {
          const th = document.createElement("th");
          th.textContent = `${col.name} (${col.dataType})`;
          headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);

        data.rows.forEach((row) => {
          const tr = document.createElement("tr");
          row.forEach((cell) => {
            const td = document.createElement("td");
            td.textContent = cell;
            tr.appendChild(td);
          });
          tbody.appendChild(tr);
        });

        previewDiv.style.display = "block";
        showLoading(false);
      })
      .catch((error) => {
        showLoading(false);
        console.error("Error:", error);
        alert(`Error generating preview: ${error.message}`);
      });
  });

  commitBtn.addEventListener("click", () => {
    const tableName = tableSelect.value || newTableName.value;
    const action = tableSelect.value
      ? document.querySelector('input[name="action"]:checked').value
      : "create";

    if (!tableName) {
      alert("Please select a table or enter a new table name.");
      return;
    }

    showLoading(true);
    const formData = new FormData();
    formData.append("csvfile", csvFile);
    formData.append("commitData", JSON.stringify({ tableName, action }));

    fetch("/commit", {
      method: "POST",
      body: formData,
    })
      .then((response) => {
        if (!response.ok)
          return response.text().then((text) => {
            throw new Error(text);
          });
        showLoading(false);
        alert("Data committed successfully!");
        optionsDiv.style.display = "none";
        previewDiv.style.display = "none";
        fileInput.value = "";
        csvFile = null;
      })
      .catch((error) => {
        showLoading(false);
        console.error("Error:", error);
        alert(`Error committing data: ${error.message}`);
      });
  });
});
